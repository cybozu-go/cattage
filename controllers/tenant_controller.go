package controllers

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	cattagev1beta1 "github.com/cybozu-go/cattage/api/v1beta1"
	"github.com/cybozu-go/cattage/pkg/accurate"
	"github.com/cybozu-go/cattage/pkg/argocd"
	extract "github.com/cybozu-go/cattage/pkg/client"
	"github.com/cybozu-go/cattage/pkg/config"
	"github.com/cybozu-go/cattage/pkg/constants"
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
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
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
//+kubebuilder:rbac:groups=argoproj.io,resources=appprojects,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;escalate;bind

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

	tenant.Status.Health = cattagev1beta1.TenantHealthy
	meta.SetStatusCondition(&tenant.Status.Conditions, metav1.Condition{
		Type:   cattagev1beta1.ConditionReady,
		Status: metav1.ConditionTrue,
		Reason: "OK",
	})
	logger.Info("Tenant successfully reconciled")

	return ctrl.Result{}, nil
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
	return nil
}

func (r *TenantReconciler) removeAppProject(ctx context.Context, tenant *cattagev1beta1.Tenant) error {
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
	return r.client.Delete(ctx, proj)
}

func (r *TenantReconciler) finalize(ctx context.Context, tenant *cattagev1beta1.Tenant) error {
	logger := log.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(tenant, constants.Finalizer) {
		return nil
	}
	logger.Info("starting finalization")
	nss := &corev1.NamespaceList{}
	if err := r.client.List(ctx, nss, client.MatchingFields{constants.RootNamespaces: tenant.Name}); err != nil {
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

	return r.client.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: constants.TenantFieldManager,
		Force:        pointer.Bool(true),
	})
}

func (r *TenantReconciler) patchRoleBinding(ctx context.Context, rb *acrbacv1.RoleBindingApplyConfiguration) error {
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

	return r.client.Patch(ctx, patch, client.Apply, &client.PatchOptions{
		FieldManager: constants.TenantFieldManager,
		Force:        pointer.Bool(true),
	})
}

func rolesMap(delegates []cattagev1beta1.DelegateSpec) map[string][]string {
	result := make(map[string][]string)

	for _, d := range delegates {
		for _, role := range d.Roles {
			result[role] = append(result[role], d.Name)
		}
	}
	return result
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
		var buf bytes.Buffer
		err = tpl.Execute(&buf, struct {
			Name  string
			Roles map[string][]string
		}{
			Name:  tenant.Name,
			Roles: rolesMap(tenant.Spec.Delegates),
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
	if err := r.client.List(ctx, nss, client.MatchingFields{constants.RootNamespaces: tenant.Name}); err != nil {
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

func (r *TenantReconciler) reconcileArgoCD(ctx context.Context, tenant *cattagev1beta1.Tenant) error {
	logger := log.FromContext(ctx)

	orig := argocd.AppProject()
	err := r.client.Get(ctx, client.ObjectKey{Namespace: r.config.ArgoCD.Namespace, Name: tenant.Name}, orig)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to get AppProject")
		return err
	}

	nss := &corev1.NamespaceList{}
	if err := r.client.List(ctx, nss, client.MatchingFields{constants.TenantNamespaces: tenant.Name}); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}
	namespaces := make([]string, len(nss.Items))
	for i, ns := range nss.Items {
		namespaces[i] = ns.Name
	}

	tpl, err := template.New("AppProject Template").Parse(r.config.ArgoCD.AppProjectTemplate)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = tpl.Execute(&buf, struct {
		Name         string
		Namespaces   []string
		Roles        map[string][]string
		Repositories []string
	}{
		Name:         tenant.Name,
		Namespaces:   namespaces,
		Roles:        rolesMap(tenant.Spec.Delegates),
		Repositories: tenant.Spec.ArgoCD.Repositories,
	})
	if err != nil {
		return err
	}

	proj := argocd.AppProject()
	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err = dec.Decode(buf.Bytes(), nil, proj)
	if err != nil {
		return err
	}

	proj.SetNamespace(r.config.ArgoCD.Namespace)
	proj.SetName(tenant.Name)
	proj.SetLabels(map[string]string{
		constants.OwnerTenant: tenant.Name,
	})

	managed, err := extract.ExtractManagedFields(orig, constants.TenantFieldManager)
	if err != nil {
		return err
	}
	if equality.Semantic.DeepEqual(proj, managed) {
		return nil
	}

	err = r.client.Patch(ctx, proj, client.Apply, &client.PatchOptions{
		Force:        pointer.BoolPtr(true),
		FieldManager: constants.TenantFieldManager,
	})
	if err != nil {
		logger.Error(err, "failed to patch AppProject")
		return err
	}
	logger.Info("AppProject successfully reconciled")

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	tenantHandler := func(o client.Object) []reconcile.Request {
		owner := o.GetLabels()[constants.OwnerTenant]
		if owner == "" {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: owner}}}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&cattagev1beta1.Tenant{}).
		Watches(&source.Kind{Type: &corev1.Namespace{}}, handler.EnqueueRequestsFromMapFunc(tenantHandler)).
		Watches(&source.Kind{Type: &rbacv1.RoleBinding{}}, handler.EnqueueRequestsFromMapFunc(tenantHandler)).
		Watches(&source.Kind{Type: argocd.AppProject()}, handler.EnqueueRequestsFromMapFunc(tenantHandler)).
		Complete(r)
}

func SetupIndexForNamespace(ctx context.Context, mgr manager.Manager) error {
	ns := &corev1.Namespace{}
	err := mgr.GetFieldIndexer().IndexField(ctx, ns, constants.RootNamespaces, func(rawObj client.Object) []string {
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

	return mgr.GetFieldIndexer().IndexField(ctx, ns, constants.TenantNamespaces, func(rawObj client.Object) []string {
		tenantName := rawObj.GetLabels()[constants.OwnerTenant]
		if tenantName == "" {
			return nil
		}
		return []string{tenantName}
	})
}
