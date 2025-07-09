package controller

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"text/template"

	cattagev1beta1 "github.com/cybozu-go/cattage/api/v1beta1"
	"github.com/cybozu-go/cattage/internal/accurate"
	"github.com/cybozu-go/cattage/internal/argocd"
	extract "github.com/cybozu-go/cattage/internal/client"
	"github.com/cybozu-go/cattage/internal/config"
	"github.com/cybozu-go/cattage/internal/constants"
	"github.com/cybozu-go/cattage/internal/metrics"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	accorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	acrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func NewTenantReconciler(client client.Client, config *config.Config) *TenantReconciler {
	return &TenantReconciler{
		client: client,
		config: config,
	}
}

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client client.Client
	config *config.Config
}

//+kubebuilder:rbac:groups=cattage.cybozu.io,resources=tenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cattage.cybozu.io,resources=tenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cattage.cybozu.io,resources=tenants/finalizers,verbs=update
//+kubebuilder:rbac:groups=cattage.cybozu.io,resources=syncwindows,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cattage.cybozu.io,resources=syncwindows/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cattage.cybozu.io,resources=syncwindows/finalizers,verbs=update
//+kubebuilder:rbac:groups=argoproj.io,resources=appprojects,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;escalate;bind
//+kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Tenant object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	needRequeue, err := r.migrateToArgoCD25(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if needRequeue {
		return ctrl.Result{Requeue: true}, nil
	}

	tenant := &cattagev1beta1.Tenant{}
	if err := r.client.Get(ctx, req.NamespacedName, tenant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if tenant.DeletionTimestamp != nil {
		if err := r.finalize(ctx, tenant); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to finalize: %w", err)
		}
		return ctrl.Result{}, nil
	}

	defer func(before cattagev1beta1.TenantStatus) {
		if !equality.Semantic.DeepEqual(tenant.Status, before) {
			logger.Info("update status", "status", tenant.Status, "before", before)
			if err2 := r.client.Status().Update(ctx, tenant); err2 != nil {
				logger.Error(err2, "failed to update status")
				err = err2
			}
		}
		r.setMetrics(tenant)
	}(tenant.Status)

	err = r.reconcileNamespaces(ctx, tenant)
	if err != nil {
		tenant.Status.Health = cattagev1beta1.TenantUnhealthy
		meta.SetStatusCondition(&tenant.Status.Conditions, metav1.Condition{
			Type:    cattagev1beta1.ConditionReady,
			Status:  metav1.ConditionFalse,
			Reason:  "Failed",
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	err = r.reconcileArgoCD(ctx, tenant)
	if err != nil {
		tenant.Status.Health = cattagev1beta1.TenantUnhealthy
		meta.SetStatusCondition(&tenant.Status.Conditions, metav1.Condition{
			Type:    cattagev1beta1.ConditionReady,
			Status:  metav1.ConditionFalse,
			Reason:  "Failed",
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	err = r.reconcileConfigMapForApplicationController(ctx, tenant)
	if err != nil {
		tenant.Status.Health = cattagev1beta1.TenantUnhealthy
		meta.SetStatusCondition(&tenant.Status.Conditions, metav1.Condition{
			Type:    cattagev1beta1.ConditionReady,
			Status:  metav1.ConditionFalse,
			Reason:  "Failed",
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	tenant.Status.Health = cattagev1beta1.TenantHealthy
	meta.SetStatusCondition(&tenant.Status.Conditions, metav1.Condition{
		Type:   cattagev1beta1.ConditionReady,
		Status: metav1.ConditionTrue,
		Reason: "OK",
	})
	logger.Info("Tenant successfully reconciled")

	return ctrl.Result{}, nil
}

func (r *TenantReconciler) migrateToArgoCD25(ctx context.Context) (bool /* needRequeue */, error) {
	apps := argocd.ApplicationList()
	if err := r.client.List(ctx, apps, client.HasLabels{constants.OwnerAppNamespace}, client.InNamespace(r.config.ArgoCD.Namespace)); err != nil {
		return false, fmt.Errorf("failed to list applications: %w", err)
	}
	if len(apps.Items) == 0 {
		return false, nil
	}

	needRequeue := false
	for _, app := range apps.Items {
		if controllerutil.ContainsFinalizer(&app, argocd.ResourcesFinalizer) {
			needRequeue = true
			controllerutil.RemoveFinalizer(&app, argocd.ResourcesFinalizer)
			err := r.client.Update(ctx, &app)
			if err != nil {
				return false, err
			}
		}
	}
	if needRequeue {
		return true, nil
	}
	for _, app := range apps.Items {
		uid := app.GetUID()
		resourceVersion := app.GetResourceVersion()
		cond := metav1.Preconditions{
			UID:             &uid,
			ResourceVersion: &resourceVersion,
		}
		err := r.client.Delete(ctx, &app, &client.DeleteOptions{
			Preconditions: &cond,
		})
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func containNamespace(roots []cattagev1beta1.RootNamespaceSpec, ns corev1.Namespace) bool {
	for _, root := range roots {
		if root.Name == ns.Name {
			return true
		}
	}
	return false
}

func (r *TenantReconciler) disownNamespace(ctx context.Context, ns *corev1.Namespace) error {
	managed, err := accorev1.ExtractNamespace(ns, constants.TenantFieldManager)
	if err != nil {
		return err
	}
	delete(managed.Labels, constants.OwnerTenant)
	for k := range r.config.Namespace.CommonLabels {
		delete(managed.Labels, k)
	}
	for k := range r.config.Namespace.CommonAnnotations {
		delete(managed.Annotations, k)
	}
	err = r.patchNamespace(ctx, managed)
	if err != nil {
		return err
	}
	return nil
}

func (r *TenantReconciler) removeRoleBinding(ctx context.Context, tenant *cattagev1beta1.Tenant, ns *corev1.Namespace) error {
	logger := log.FromContext(ctx)
	rb := &rbacv1.RoleBinding{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: tenant.Name + "-admin"}, rb)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if rb.DeletionTimestamp != nil {
		return nil
	}
	if err != nil {
		return err
	}
	labels := rb.GetLabels()
	if labels == nil || labels[constants.OwnerTenant] != tenant.Name {
		return nil
	}
	err = r.client.Delete(ctx, rb)
	if err != nil {
		return err
	}
	logger.Info("RoleBinding deleted", "rolebinding", rb.Name)
	return nil
}

func (r *TenantReconciler) removeAppProject(ctx context.Context, tenant *cattagev1beta1.Tenant) error {
	logger := log.FromContext(ctx)
	proj := argocd.AppProject()
	err := r.client.Get(ctx, client.ObjectKey{Namespace: r.config.ArgoCD.Namespace, Name: tenant.Name}, proj)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if proj.GetDeletionTimestamp() != nil {
		return nil
	}
	if err != nil {
		return err
	}
	labels := proj.GetLabels()
	if labels == nil || labels[constants.OwnerTenant] != tenant.Name {
		return nil
	}
	err = r.client.Delete(ctx, proj)
	if err != nil {
		return err
	}
	logger.Info("AppProject deleted", "project", proj.GetName())
	return nil
}

func (r *TenantReconciler) finalize(ctx context.Context, tenant *cattagev1beta1.Tenant) error {
	logger := log.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(tenant, constants.Finalizer) {
		return nil
	}
	logger.Info("starting finalization")
	nss := &corev1.NamespaceList{}
	if err := r.client.List(ctx, nss, client.MatchingFields{constants.RootNamespaceIndex: tenant.Name}); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}
	for _, ns := range nss.Items {
		err := r.disownNamespace(ctx, &ns)
		if err != nil {
			return err
		}
		err = r.removeRoleBinding(ctx, tenant, &ns)
		if err != nil {
			return err
		}
	}
	err := r.removeAppProject(ctx, tenant)
	if err != nil {
		return err
	}

	r.removeMetrics(tenant)
	controllerutil.RemoveFinalizer(tenant, constants.Finalizer)
	err = r.client.Update(ctx, tenant)
	if err != nil {
		logger.Error(err, "failed to remove finalizer")
		return err
	}
	logger.Info("finished finalization")
	return nil
}

func (r *TenantReconciler) patchNamespace(ctx context.Context, ns *accorev1.NamespaceApplyConfiguration) error {
	logger := log.FromContext(ctx)
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ns)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var orig corev1.Namespace
	err = r.client.Get(ctx, client.ObjectKey{Name: *ns.Name}, &orig)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	managed, err := accorev1.ExtractNamespace(&orig, constants.TenantFieldManager)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(ns, managed) {
		return nil
	}

	logger.Info("patching namespace", "namespace", ns, "managed", managed)
	return r.client.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: constants.TenantFieldManager,
		Force:        ptr.To(true),
	})
}

func (r *TenantReconciler) patchRoleBinding(ctx context.Context, rb *acrbacv1.RoleBindingApplyConfiguration) error {
	logger := log.FromContext(ctx)
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rb)
	if err != nil {
		return err
	}
	patch := &unstructured.Unstructured{
		Object: obj,
	}

	var orig rbacv1.RoleBinding
	err = r.client.Get(ctx, client.ObjectKey{Namespace: *rb.Namespace, Name: *rb.Name}, &orig)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	managed, err := acrbacv1.ExtractRoleBinding(&orig, constants.TenantFieldManager)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(rb, managed) {
		return nil
	}

	logger.Info("patching RoleBinding", "rolebinding", rb, "managed", managed)
	return r.client.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: constants.TenantFieldManager,
		Force:        ptr.To(true),
	})
}

func (r *TenantReconciler) rolesMap(ctx context.Context, delegates []cattagev1beta1.DelegateSpec) (map[string][]Role, error) {
	result := make(map[string][]Role)

	for _, d := range delegates {
		delegatedTenant := &cattagev1beta1.Tenant{}
		err := r.client.Get(ctx, client.ObjectKey{Name: d.Name}, delegatedTenant)
		if err != nil {
			return nil, err
		}
		for _, role := range d.Roles {
			result[role] = append(result[role], Role{
				Name:        delegatedTenant.Name,
				ExtraParams: delegatedTenant.Spec.ExtraParams.ToMap(),
			})
		}
	}
	for _, roles := range result {
		slices.SortFunc(roles, func(x, y Role) int {
			return cmp.Compare(x.Name, y.Name)
		})
	}
	return result, nil
}

func (r *TenantReconciler) reconcileNamespaces(ctx context.Context, tenant *cattagev1beta1.Tenant) error {
	for _, ns := range tenant.Spec.RootNamespaces {
		namespace := accorev1.Namespace(ns.Name)
		labels := make(map[string]string)
		for k, v := range r.config.Namespace.CommonLabels {
			labels[k] = v
		}
		for k, v := range ns.Labels {
			labels[k] = v
		}
		labels[accurate.LabelType] = accurate.NSTypeRoot
		labels[constants.OwnerTenant] = tenant.Name
		namespace.WithLabels(labels)
		annotations := make(map[string]string)
		for k, v := range r.config.Namespace.CommonAnnotations {
			annotations[k] = v
		}
		for k, v := range ns.Annotations {
			annotations[k] = v
		}
		namespace.WithAnnotations(annotations)
		err := r.patchNamespace(ctx, namespace)
		if err != nil {
			return err
		}

		tpl, err := template.New("RoleBinding Template").Parse(r.config.Namespace.RoleBindingTemplate)
		if err != nil {
			return err
		}
		roles, err := r.rolesMap(ctx, tenant.Spec.Delegates)
		if err != nil {
			return err
		}

		var buf bytes.Buffer
		err = tpl.Execute(&buf, struct {
			Name        string
			Roles       map[string][]Role
			ExtraParams map[string]interface{}
		}{
			Name:        tenant.Name,
			Roles:       roles,
			ExtraParams: tenant.Spec.ExtraParams.ToMap(),
		})
		if err != nil {
			return err
		}

		rb := acrbacv1.RoleBinding(tenant.Name+"-admin", ns.Name)
		err = k8syaml.Unmarshal(buf.Bytes(), rb)
		if err != nil {
			return err
		}
		rb.WithLabels(map[string]string{
			constants.OwnerTenant: tenant.Name,
		})
		rb.WithAnnotations(map[string]string{
			accurate.AnnPropagate: accurate.PropagateUpdate,
		})

		err = r.patchRoleBinding(ctx, rb)
		if err != nil {
			return err
		}
	}
	nss := &corev1.NamespaceList{}
	if err := r.client.List(ctx, nss, client.MatchingFields{constants.RootNamespaceIndex: tenant.Name}); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}
	for _, ns := range nss.Items {
		if containNamespace(tenant.Spec.RootNamespaces, ns) {
			continue
		}
		err := r.disownNamespace(ctx, &ns)
		if err != nil {
			return err
		}
		err = r.removeRoleBinding(ctx, tenant, &ns)
		if err != nil {
			return err
		}
	}

	return nil
}

type Role struct {
	Name        string
	ExtraParams map[string]interface{}
}

func (r *TenantReconciler) reconcileArgoCD(ctx context.Context, tenant *cattagev1beta1.Tenant) error {
	logger := log.FromContext(ctx)

	orig := argocd.AppProject()
	err := r.client.Get(ctx, client.ObjectKey{Namespace: r.config.ArgoCD.Namespace, Name: tenant.Name}, orig)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to get AppProject")
		return err
	}

	tpl, err := template.New("AppProject Template").Parse(r.config.ArgoCD.AppProjectTemplate)
	if err != nil {
		return err
	}

	namespaces, err := r.getTenantNamespaces(ctx, tenant)
	if err != nil {
		return err
	}

	roles, err := r.rolesMap(ctx, tenant.Spec.Delegates)
	if err != nil {
		return err
	}

	repos := tenant.Spec.ArgoCD.Repositories
	slices.Sort(repos)
	params := tenant.Spec.ExtraParams.ToMap()

	var buf bytes.Buffer
	err = tpl.Execute(&buf, struct {
		Name         string
		Namespaces   []string
		Roles        map[string][]Role
		Repositories []string
		ExtraParams  map[string]interface{}
	}{
		Name:         tenant.Name,
		Namespaces:   namespaces,
		Roles:        roles,
		Repositories: repos,
		ExtraParams:  params,
	})
	if err != nil {
		return err
	}

	proj := argocd.AppProject()
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err = dec.Decode(buf.Bytes(), nil, proj)
	if err != nil {
		logger.Error(err, "failed to decode", "yaml", buf.String())
		return err
	}

	proj.SetNamespace(r.config.ArgoCD.Namespace)
	proj.SetName(tenant.Name)
	proj.SetLabels(map[string]string{
		constants.OwnerTenant: tenant.Name,
	})
	val, found, err := unstructured.NestedSlice(proj.UnstructuredContent(), "spec", "syncWindows")
	if err != nil {
		return err
	}
	var syncWindows cattagev1beta1.SyncWindows
	if !found {
		syncWindows = cattagev1beta1.SyncWindows{}
	} else {
		syncWindows, err = fromUnstructuredSlice[cattagev1beta1.SyncWindows](val)
		if err != nil {
			return err
		}
	}
	swResources, sws, err := r.getSyncWindows(ctx, tenant.Name)
	if err != nil {
		return fmt.Errorf("failed to get sync windows: %w", err)
	}
	syncWindows = append(syncWindows, sws...)
	if len(syncWindows) != 0 {
		ret, err := toUnstructuredSlice[cattagev1beta1.SyncWindows](syncWindows)
		if err != nil {
			return err
		}
		err = unstructured.SetNestedSlice(proj.UnstructuredContent(), ret, "spec", "syncWindows")
		if err != nil {
			return err
		}
	}

	managed, err := extract.ExtractManagedFields(orig, constants.TenantFieldManager)
	if err != nil {
		return err
	}
	if equality.Semantic.DeepEqual(proj, managed) && allSyncWindowsAreSynced(swResources) {
		return nil
	}

	logger.Info("patching AppProject", "namespaces", namespaces, "roles", roles, "repositories", repos, "extraParams", params)
	err = r.client.Patch(ctx, proj, client.Apply, &client.PatchOptions{
		Force:        ptr.To(true),
		FieldManager: constants.TenantFieldManager,
	})
	if err != nil {
		logger.Error(err, "failed to patch AppProject")
		return err
	}

	err = r.updateSyncWindowStatus(ctx, swResources)
	if err != nil {
		return err
	}

	logger.Info("AppProject successfully reconciled")

	return nil
}

func (r *TenantReconciler) getTenantNamespaces(ctx context.Context, tenant *cattagev1beta1.Tenant) ([]string, error) {
	nss := &corev1.NamespaceList{}
	if err := r.client.List(ctx, nss, client.MatchingFields{constants.TenantNamespaceIndex: tenant.Name}); err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}
	namespaces := make([]string, len(nss.Items))
	for i, ns := range nss.Items {
		namespaces[i] = ns.Name
	}
	delegatedNamespaces, err := r.getDelegatedNamespaces(ctx, tenant.Spec.Delegates)
	if err != nil {
		return nil, err
	}
	namespaces = append(namespaces, delegatedNamespaces...)
	slices.Sort(namespaces)
	return namespaces, nil
}

func (r *TenantReconciler) getSyncWindows(ctx context.Context, tenantOwner string) ([]cattagev1beta1.SyncWindow, cattagev1beta1.SyncWindows, error) {
	nss := &corev1.NamespaceList{}
	if err := r.client.List(ctx, nss, client.MatchingFields{constants.TenantNamespaceIndex: tenantOwner}); err != nil {
		return nil, nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	resources := make([]cattagev1beta1.SyncWindow, 0)
	for _, ns := range nss.Items {
		sws := &cattagev1beta1.SyncWindowList{}
		err := r.client.List(ctx, sws, client.InNamespace(ns.Name))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list sync windows in namespace %s: %w", ns.Name, err)
		}
		resources = append(resources, sws.Items...)
	}

	slices.SortFunc(resources, func(x, y cattagev1beta1.SyncWindow) int {
		if x.Namespace != y.Namespace {
			return cmp.Compare(x.Namespace, y.Namespace)
		}
		return cmp.Compare(x.Name, y.Name)
	})

	syncWindows := cattagev1beta1.SyncWindows{}

	for _, res := range resources {
		syncWindows = append(syncWindows, res.Spec.SyncWindows...)
	}

	return resources, syncWindows, nil
}

func (r *TenantReconciler) updateSyncWindowStatus(ctx context.Context, resources []cattagev1beta1.SyncWindow) error {
	errs := make([]error, 0)
	for _, res := range resources {
		meta.SetStatusCondition(&res.Status.Conditions, metav1.Condition{
			Type:   cattagev1beta1.ConditionSynced,
			Status: metav1.ConditionTrue,
			Reason: "OK",
		})
		err := r.client.Status().Update(ctx, &res)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to update sync window status: %v", errs)
	}
	return nil
}

func allSyncWindowsAreSynced(resources []cattagev1beta1.SyncWindow) bool {
	for _, res := range resources {
		if !meta.IsStatusConditionTrue(res.Status.Conditions, cattagev1beta1.ConditionSynced) {
			return false
		}
	}
	return true
}

func fromUnstructuredSlice[T any](data []interface{}) (T, error) {
	var t T
	b, err := json.Marshal(data)
	if err != nil {
		return t, err
	}
	err = json.Unmarshal(b, &t)
	return t, err
}

func toUnstructuredSlice[T any](obj T) ([]interface{}, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var m []interface{}
	err = json.Unmarshal(b, &m)
	return m, err
}

func (r *TenantReconciler) getDelegatedNamespaces(ctx context.Context, delegates []cattagev1beta1.DelegateSpec) ([]string, error) {
	result := make([]string, 0)

	for _, d := range delegates {
		nss := &corev1.NamespaceList{}
		if err := r.client.List(ctx, nss, client.MatchingFields{constants.TenantNamespaceIndex: d.Name}); err != nil {
			return nil, err
		}
		for _, ns := range nss.Items {
			result = append(result, ns.Name)
		}
	}
	return result, nil
}

func (r *TenantReconciler) reconcileConfigMapForApplicationController(ctx context.Context, tenant *cattagev1beta1.Tenant) error {
	cmList := &corev1.ConfigMapList{}
	err := r.client.List(ctx, cmList, client.MatchingLabels{constants.ManagedByLabel: "cattage"})
	if err != nil {
		return err
	}
	controllerNames := map[string]struct{}{}
	for _, cm := range cmList.Items {
		if cm.Labels[constants.ControllerNameLabel] != "" {
			controllerNames[cm.Labels[constants.ControllerNameLabel]] = struct{}{}
		}
	}
	controllerName := tenant.Spec.ControllerName
	if controllerName == "" {
		controllerName = constants.DefaultApplicationControllerName
	}
	controllerNames[controllerName] = struct{}{}

	for name := range controllerNames {
		err := r.updateConfigMap(ctx, name)
		if err != nil {
			return err
		}
	}

	err = r.updateAllTenantNamespacesConfigMap(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *TenantReconciler) updateConfigMap(ctx context.Context, controllerName string) error {
	logger := log.FromContext(ctx)

	configMapName := controllerName + "-application-controller-cm"
	cm := &corev1.ConfigMap{}
	cm.Name = configMapName
	cm.Namespace = r.config.ArgoCD.Namespace

	tenantList := &cattagev1beta1.TenantList{}
	if err := r.client.List(ctx, tenantList, client.MatchingFields{constants.ControllerNameIndex: controllerName}); err != nil {
		return fmt.Errorf("failed to list tenants: %w", err)
	}

	tenants := tenantList.Items
	slices.SortFunc(tenants, func(x, y cattagev1beta1.Tenant) int {
		return cmp.Compare(x.Name, y.Name)
	})

	if len(tenants) == 0 {
		err := r.client.Delete(ctx, cm)
		return err
	}

	namespaces := make([]string, 0)
	for _, t := range tenants {
		nss := &corev1.NamespaceList{}
		if err := r.client.List(ctx, nss, client.MatchingFields{constants.TenantNamespaceIndex: t.Name}); err != nil {
			return fmt.Errorf("failed to list namespaces: %w", err)
		}
		for _, ns := range nss.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}
	slices.Sort(namespaces)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, cm, func() error {
		cm.Labels = map[string]string{
			constants.ManagedByLabel:      "cattage",
			constants.PartOfLabel:         "argocd",
			constants.ControllerNameLabel: controllerName,
		}
		cm.Data = map[string]string{
			"application.namespaces": strings.Join(namespaces, ","),
		}
		cm.OwnerReferences = nil
		for _, tenant := range tenants {
			err := controllerutil.SetOwnerReference(&tenant, cm, r.client.Scheme())
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		logger.Error(err, "failed to update ConfigMap")
		return err
	}
	if op != controllerutil.OperationResultNone {
		tenantNames := make([]string, len(tenants))
		for i, t := range tenants {
			tenantNames[i] = t.Name
		}
		logger.Info("ConfigMap successfully reconciled", "namespaces", namespaces, "tenants", tenantNames)
	}

	return nil
}

func (r *TenantReconciler) updateAllTenantNamespacesConfigMap(ctx context.Context) error {
	logger := log.FromContext(ctx)

	configMapName := "all-tenant-namespaces-cm"
	cm := &corev1.ConfigMap{}
	cm.Name = configMapName
	cm.Namespace = r.config.ArgoCD.Namespace

	tenantList := &cattagev1beta1.TenantList{}
	err := r.client.List(ctx, tenantList)
	if err != nil {
		return err
	}
	tenants := tenantList.Items
	slices.SortFunc(tenants, func(x, y cattagev1beta1.Tenant) int {
		return cmp.Compare(x.Name, y.Name)
	})

	allNamespaces := make([]string, 0)
	for _, tenant := range tenants {
		nss := &corev1.NamespaceList{}
		if err := r.client.List(ctx, nss, client.MatchingFields{constants.TenantNamespaceIndex: tenant.Name}); err != nil {
			return fmt.Errorf("failed to list namespaces: %w", err)
		}
		for _, ns := range nss.Items {
			allNamespaces = append(allNamespaces, ns.Name)
		}
	}
	slices.Sort(allNamespaces)

	op, err := ctrl.CreateOrUpdate(ctx, r.client, cm, func() error {
		cm.Labels = map[string]string{
			constants.ManagedByLabel: "cattage",
			constants.PartOfLabel:    "argocd",
		}
		cm.Data = map[string]string{
			"application.namespaces": strings.Join(allNamespaces, ","),
		}
		cm.OwnerReferences = nil
		for _, tenant := range tenants {
			err := controllerutil.SetOwnerReference(&tenant, cm, r.client.Scheme())
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		logger.Error(err, "failed to update ConfigMap")
		return err
	}
	if op != controllerutil.OperationResultNone {
		tenantNames := make([]string, len(tenants))
		for i, t := range tenants {
			tenantNames[i] = t.Name
		}
		logger.Info("ConfigMap successfully reconciled", "allNamespaces", allNamespaces, "tenants", tenantNames)
	}

	return nil
}

func (r *TenantReconciler) setMetrics(tenant *cattagev1beta1.Tenant) {
	switch tenant.Status.Health {
	case cattagev1beta1.TenantHealthy:
		metrics.HealthyVec.WithLabelValues(tenant.Name).Set(1)
		metrics.UnhealthyVec.WithLabelValues(tenant.Name).Set(0)
	case cattagev1beta1.TenantUnhealthy:
		metrics.HealthyVec.WithLabelValues(tenant.Name).Set(0)
		metrics.UnhealthyVec.WithLabelValues(tenant.Name).Set(1)
	}
}

func (r *TenantReconciler) removeMetrics(tenant *cattagev1beta1.Tenant) {
	metrics.HealthyVec.DeleteLabelValues(tenant.Name)
	metrics.UnhealthyVec.DeleteLabelValues(tenant.Name)
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	tenantHandler := func(ctx context.Context, o client.Object) []reconcile.Request {
		owner := o.GetLabels()[constants.OwnerTenant]
		if owner == "" {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: owner}}}
	}
	nsHandler := func(ctx context.Context, o client.Object) []reconcile.Request {
		ns := &corev1.Namespace{}
		err := r.client.Get(ctx, client.ObjectKey{Name: o.GetNamespace()}, ns)
		if err != nil {
			logger := log.FromContext(ctx)
			logger.Error(err, "failed to get namespace", "namespace", o.GetNamespace())
			return nil
		}

		owner := ns.GetLabels()[constants.OwnerTenant]
		if owner == "" {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: owner}}}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&cattagev1beta1.Tenant{}).
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(tenantHandler)).
		Watches(&rbacv1.RoleBinding{}, handler.EnqueueRequestsFromMapFunc(tenantHandler)).
		Watches(argocd.AppProject(), handler.EnqueueRequestsFromMapFunc(tenantHandler)).
		Watches(&cattagev1beta1.SyncWindow{}, handler.EnqueueRequestsFromMapFunc(nsHandler)).
		Complete(r)
}

func SetupIndexForNamespace(ctx context.Context, mgr manager.Manager) error {
	ns := &corev1.Namespace{}
	err := mgr.GetFieldIndexer().IndexField(ctx, ns, constants.RootNamespaceIndex, func(rawObj client.Object) []string {
		nsType := rawObj.GetLabels()[accurate.LabelType]
		if nsType != accurate.NSTypeRoot {
			return nil
		}
		tenantName := rawObj.GetLabels()[constants.OwnerTenant]
		if tenantName == "" {
			return nil
		}
		return []string{tenantName}
	})
	if err != nil {
		return err
	}

	err = mgr.GetFieldIndexer().IndexField(ctx, ns, constants.TenantNamespaceIndex, func(rawObj client.Object) []string {
		tenantName := rawObj.GetLabels()[constants.OwnerTenant]
		if tenantName == "" {
			return nil
		}
		return []string{tenantName}
	})
	if err != nil {
		return err
	}

	tenant := &cattagev1beta1.Tenant{}
	return mgr.GetFieldIndexer().IndexField(ctx, tenant, constants.ControllerNameIndex, func(rawObj client.Object) []string {
		tenant := rawObj.(*cattagev1beta1.Tenant)
		controllerName := tenant.Spec.ControllerName
		if controllerName == "" {
			return []string{constants.DefaultApplicationControllerName}
		}
		return []string{controllerName}
	})
}
