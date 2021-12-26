package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cybozu-go/neco-tenant-controller/pkg/argocd"
	extract "github.com/cybozu-go/neco-tenant-controller/pkg/client"
	"github.com/cybozu-go/neco-tenant-controller/pkg/config"
	"github.com/cybozu-go/neco-tenant-controller/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
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
)

func NewApplicationReconciler(client client.Client, recorder record.EventRecorder, config *config.Config) *ApplicationReconciler {
	return &ApplicationReconciler{
		client:   client,
		recorder: recorder,
		config:   config,
	}
}

// ApplicationReconciler reconciles an Application object
type ApplicationReconciler struct {
	client   client.Client
	recorder record.EventRecorder
	config   *config.Config
}

//+kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

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

	removed, err := r.fixProject(ctx, argocdApp, tenantApp)
	if err != nil {
		logger.Error(err, "failed to validate application project")
		return err
	}
	if removed {
		return nil
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

func (r *ApplicationReconciler) fixProject(ctx context.Context, argocdApp *unstructured.Unstructured, tenantApp *unstructured.Unstructured) (removed bool, err error) {
	logger := log.FromContext(ctx)

	ns := &corev1.Namespace{}
	err = r.client.Get(ctx, client.ObjectKey{Name: tenantApp.GetNamespace()}, ns)
	if err != nil {
		return
	}
	tenantName := ns.Labels[constants.OwnerTenant]
	if tenantName == "" {
		if argocdApp != nil && argocdApp.GetDeletionTimestamp() == nil {
			logger.Info("Remove unmanaged application")
			err = r.client.Delete(ctx, argocdApp)
			if err != nil {
				r.recorder.Eventf(tenantApp, corev1.EventTypeWarning, "RemoveApplicationFailed", "Failed to remove unmanaged application", err)
				return
			}
			r.recorder.Eventf(tenantApp, corev1.EventTypeNormal, "ApplicationRemoved", "Remove unmanaged application succeeded")
		}
		removed = true
		return
	}
	project, found, err := unstructured.NestedString(tenantApp.UnstructuredContent(), "spec", "project")
	if err != nil {
		return
	}
	if !found {
		err = errors.New("spec.project not found")
		return
	}
	if project != tenantName {
		logger.Info("Overwrite project", "before", project, "after", tenantName)
		newApp := argocd.Application()
		newApp.SetNamespace(tenantApp.GetNamespace())
		newApp.SetName(tenantApp.GetName())
		err = unstructured.SetNestedField(newApp.UnstructuredContent(), tenantName, "spec", "project")
		if err != nil {
			return
		}
		err = r.client.Patch(ctx, newApp, client.Apply, &client.PatchOptions{
			Force:        pointer.BoolPtr(true),
			FieldManager: constants.ProjectFieldManager,
		})
		if err != nil {
			r.recorder.Eventf(tenantApp, corev1.EventTypeWarning, "FixProjectFailed", "Failed to fix application project", err)
			return
		}
		r.recorder.Eventf(tenantApp, corev1.EventTypeNormal, "ProjectFixed", "Fix application project succeeded")
		return
	}
	return
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
		managed, err := extract.ExtractManagedFields(argocdApp, constants.SpecFieldManager)
		if err != nil {
			logger.Error(err, "failed to extract managed fields")
			return err
		}
		if equality.Semantic.DeepEqual(managed, newApp.UnstructuredContent()) {
			return nil
		}
	}

	err := r.client.Patch(ctx, newApp, client.Apply, &client.PatchOptions{
		Force:        pointer.BoolPtr(true),
		FieldManager: constants.SpecFieldManager,
	})
	if err != nil {
		r.recorder.Eventf(tenantApp, corev1.EventTypeWarning, "SyncSpecFailed", "Failed to sync application spec", err)
		return err
	}
	r.recorder.Eventf(tenantApp, corev1.EventTypeNormal, "ApplicationSynced", "Sync application spec succeeded")
	return nil
}

func (r *ApplicationReconciler) syncApplicationStatus(ctx context.Context, argocdApp *unstructured.Unstructured, tenantApp *unstructured.Unstructured) error {
	logger := log.FromContext(ctx)

	newApp := argocd.Application()
	newApp.SetNamespace(tenantApp.GetNamespace())
	newApp.SetName(tenantApp.GetName())
	if argocdApp != nil && argocdApp.UnstructuredContent()["status"] != nil {
		newApp.UnstructuredContent()["status"] = argocdApp.DeepCopy().UnstructuredContent()["status"]
	}

	managed, err := extract.ExtractManagedFields(tenantApp, constants.StatusFieldManager)
	if err != nil {
		logger.Error(err, "failed to extract managed fields")
		return err
	}
	if equality.Semantic.DeepEqual(managed, newApp.UnstructuredContent()) {
		return nil
	}

	// MEMO: Use `r.Patch` instead of `r.Status().Patch()`, because the status of application is not a sub-resource.
	err = r.client.Patch(ctx, newApp, client.Apply, &client.PatchOptions{
		Force:        pointer.BoolPtr(true),
		FieldManager: constants.StatusFieldManager,
	})
	if err != nil {
		r.recorder.Eventf(tenantApp, corev1.EventTypeWarning, "SyncStatusFailed", "Failed to sync application status", err)
		return err
	}
	r.recorder.Eventf(tenantApp, corev1.EventTypeNormal, "StatusSynced", "Sync application status succeeded")
	return nil
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
