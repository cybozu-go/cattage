package controllers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cybozu-go/neco-tenant-controller/pkg/argocd"
	"github.com/cybozu-go/neco-tenant-controller/pkg/config"
	"github.com/cybozu-go/neco-tenant-controller/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/typed"
)

func NewApplicationReconciler(client client.Client, config *config.Config) *ApplicationReconciler {
	return &ApplicationReconciler{
		client: client,
		config: config,
	}
}

// ApplicationReconciler reconciles an Application object
type ApplicationReconciler struct {
	client client.Client
	config *config.Config
}

//+kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch;create;update;patch;delete

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	app := argocd.Application()
	if err := r.client.Get(ctx, req.NamespacedName, app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var argocdApp *unstructured.Unstructured
	var tenantApp *unstructured.Unstructured
	if req.Namespace == r.config.ArgoCD.Namespace {
		if app.GetDeletionTimestamp() != nil {
			return ctrl.Result{}, nil
		}
		argocdApp = app
		ownerNs := argocdApp.GetLabels()[constants.OwnerAppNamespace]
		if len(ownerNs) == 0 {
			return ctrl.Result{}, nil
		}
		tenantApp = argocd.Application()
		err := r.client.Get(ctx, client.ObjectKey{Namespace: ownerNs, Name: argocdApp.GetName()}, tenantApp)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		tenantApp = app
		argocdApp = argocd.Application()
		err := r.client.Get(ctx, client.ObjectKey{Namespace: r.config.ArgoCD.Namespace, Name: tenantApp.GetName()}, argocdApp)
		if err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		if apierrors.IsNotFound(err) {
			argocdApp = nil
		}

		if tenantApp.GetDeletionTimestamp() != nil {
			res, err := r.finalize(ctx, argocdApp, tenantApp)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to finalize: %w", err)
			}
			return res, nil
		}
	}

	err := r.reconcileApplication(ctx, argocdApp, tenantApp)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ApplicationReconciler) finalize(ctx context.Context, argocdApp *unstructured.Unstructured, tenantApp *unstructured.Unstructured) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if !controllerutil.ContainsFinalizer(tenantApp, constants.Finalizer) &&
		!controllerutil.ContainsFinalizer(tenantApp, argocd.ResourcesFinalizer) {
		return ctrl.Result{}, nil
	}
	if argocdApp == nil {
		controllerutil.RemoveFinalizer(tenantApp, constants.Finalizer)
		controllerutil.RemoveFinalizer(tenantApp, argocd.ResourcesFinalizer)
		err := r.client.Update(ctx, tenantApp)
		if err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("finished finalization")
		return ctrl.Result{}, nil
	}
	if argocdApp.GetDeletionTimestamp() != nil {
		return ctrl.Result{Requeue: true}, nil
	}

	logger.Info("starting finalization")
	err := r.client.Delete(ctx, argocdApp)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *ApplicationReconciler) reconcileApplication(ctx context.Context, argocdApp *unstructured.Unstructured, tenantApp *unstructured.Unstructured) error {
	logger := log.FromContext(ctx)

	err := r.validateProject(ctx, tenantApp)
	if err != nil {
		logger.Error(err, "failed to validate application project")
		return err
	}

	err = r.syncApplicationSpec(ctx, argocdApp, tenantApp)
	if err != nil {
		logger.Error(err, "failed to sync application spec")
		return err
	}
	err = r.syncApplicationStatus(ctx, argocdApp, tenantApp)
	if err != nil {
		logger.Error(err, "failed to sync application status")
		return err
	}
	return nil
}

func (r *ApplicationReconciler) validateProject(ctx context.Context, tenantApp *unstructured.Unstructured) error {
	logger := log.FromContext(ctx)

	ns := &corev1.Namespace{}
	err := r.client.Get(ctx, client.ObjectKey{Name: tenantApp.GetNamespace()}, ns)
	if err != nil {
		return err
	}
	group := ns.Labels[r.config.Namespace.GroupKey]
	if group == "" {
		logger.Info("Remove unmanaged application")
		return r.client.Delete(ctx, tenantApp)
	}
	project, found, err := unstructured.NestedString(tenantApp.UnstructuredContent(), "spec", "project")
	if err != nil {
		return err
	}
	if !found {
		return errors.New("spec.project not found")
	}
	if project != group {
		logger.Info("Overwrite project", "before", project, "after", group)
		err := unstructured.SetNestedField(tenantApp.UnstructuredContent(), group, "spec", "project")
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ApplicationReconciler) extractManagedFields(u *unstructured.Unstructured, manager string) (map[string]interface{}, error) {
	fieldset := &fieldpath.Set{}
	objManagedFields := u.GetManagedFields()
	for _, mf := range objManagedFields {
		if mf.Manager != manager || mf.Operation != metav1.ManagedFieldsOperationApply {
			continue
		}
		fs := &fieldpath.Set{}
		err := fs.FromJSON(bytes.NewReader(mf.FieldsV1.Raw))
		if err != nil {
			return nil, err
		}
		fieldset = fieldset.Union(fs)
	}

	d, err := typed.DeducedParseableType.FromUnstructured(u.Object)
	if err != nil {
		return nil, err
	}

	x := d.ExtractItems(fieldset.Leaves()).AsValue().Unstructured()
	m, ok := x.(map[string]interface{})
	if !ok {
		return nil, errors.New("cannot cast")
	}

	m["apiVersion"] = "argoproj.io/" + argocd.ApplicationVersion
	m["kind"] = "Application"
	m["metadata"].(map[string]interface{})["name"] = u.GetName()
	m["metadata"].(map[string]interface{})["namespace"] = r.config.ArgoCD.Namespace
	return m, nil
}

func (r *ApplicationReconciler) syncApplicationSpec(ctx context.Context, argocdApp *unstructured.Unstructured, tenantApp *unstructured.Unstructured) error {
	logger := log.FromContext(ctx)

	labels := make(map[string]string)
	for k, v := range tenantApp.GetLabels() {
		if strings.Contains(k, "kubernetes.io/") {
			continue
		}
		labels[k] = v
	}
	labels[constants.OwnerAppNamespace] = tenantApp.GetNamespace()

	annotations := make(map[string]string)
	for k, v := range tenantApp.GetAnnotations() {
		if strings.Contains(k, "kubernetes.io/") {
			continue
		}
		annotations[k] = v
	}
	var finalizers []string
	for _, fin := range tenantApp.GetFinalizers() {
		if fin == argocd.ResourcesFinalizer {
			finalizers = append(finalizers, fin)
		}
	}

	newApp := argocd.Application()
	newApp.UnstructuredContent()["spec"] = tenantApp.DeepCopy().UnstructuredContent()["spec"]
	newApp.SetName(tenantApp.GetName())
	newApp.SetNamespace(r.config.ArgoCD.Namespace)
	if len(labels) != 0 {
		newApp.SetLabels(labels)
	}
	if len(annotations) != 0 {
		newApp.SetAnnotations(annotations)
	}
	if len(finalizers) != 0 {
		newApp.SetFinalizers(finalizers)
	}

	if argocdApp != nil {
		managed, err := r.extractManagedFields(argocdApp, constants.FieldManager)
		if err != nil {
			logger.Error(err, "failed to extract managed fields")
			return err
		}
		if equality.Semantic.DeepEqual(managed, newApp.UnstructuredContent()) {
			return nil
		}
	}

	return r.client.Patch(ctx, newApp, client.Apply, &client.PatchOptions{
		Force:        pointer.BoolPtr(true),
		FieldManager: constants.FieldManager,
	})
}

func (r *ApplicationReconciler) syncApplicationStatus(ctx context.Context, argocdApp *unstructured.Unstructured, tenantApp *unstructured.Unstructured) error {
	if argocdApp == nil ||
		argocdApp.UnstructuredContent()["status"] == nil ||
		equality.Semantic.DeepEqual(argocdApp.UnstructuredContent()["status"], tenantApp.UnstructuredContent()["status"]) {
		return nil
	}

	newApp := argocd.Application()
	newApp.SetNamespace(tenantApp.GetNamespace())
	newApp.SetName(tenantApp.GetName())
	newApp.UnstructuredContent()["spec"] = tenantApp.DeepCopy().UnstructuredContent()["spec"]
	newApp.UnstructuredContent()["status"] = argocdApp.DeepCopy().UnstructuredContent()["status"]

	// MEMO: Use `r.Patch` instead of `r.Status().Patch()`, because the status of application is not a sub-resource.
	return r.client.Patch(ctx, newApp, client.Apply, &client.PatchOptions{
		Force:        pointer.BoolPtr(true),
		FieldManager: constants.FieldManager,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	logger := log.FromContext(ctx)

	nsHandler := func(o client.Object, q workqueue.RateLimitingInterface) {
		apps := argocd.ApplicationList()
		err := mgr.GetClient().List(ctx, apps, client.InNamespace(o.GetName()))
		if err != nil {
			logger.Error(err, "failed to list applications")
			return
		}
		for _, app := range apps.Items {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: app.GetNamespace(),
				Name:      app.GetName(),
			}})
		}
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(argocd.Application()).
		Watches(&source.Kind{Type: &corev1.Namespace{}}, handler.Funcs{
			UpdateFunc: func(ev event.UpdateEvent, q workqueue.RateLimitingInterface) {
				if ev.ObjectNew.GetDeletionTimestamp() != nil {
					return
				}
				nsHandler(ev.ObjectOld, q)
			},
		}).
		Complete(r)
}
