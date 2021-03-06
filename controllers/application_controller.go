package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cybozu-go/cattage/pkg/argocd"
	extract "github.com/cybozu-go/cattage/pkg/client"
	"github.com/cybozu-go/cattage/pkg/config"
	"github.com/cybozu-go/cattage/pkg/constants"
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
	ch := make(chan event.GenericEvent)

	return &ApplicationReconciler{
		client:   client,
		recorder: recorder,
		config:   config,
		channel:  ch,
	}
}

// ApplicationReconciler reconciles an Application object
type ApplicationReconciler struct {
	client   client.Client
	recorder record.EventRecorder
	config   *config.Config
	channel  chan event.GenericEvent
}

//+kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;update;patch

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	app := argocd.Application()

	err := r.client.Get(ctx, req.NamespacedName, app)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) && req.Namespace == r.config.ArgoCD.Namespace {
		logger.Info("argocd application was deleted")
		apps := argocd.ApplicationList()
		err = r.client.List(ctx, apps)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		for _, ap := range apps.Items {
			if ap.GetNamespace() != r.config.ArgoCD.Namespace && ap.GetName() == req.Name {
				logger.Info("queue the tenant application", "tenantApp", ap)
				r.channel <- event.GenericEvent{
					Object: ap.DeepCopy(),
				}
				break
			}
		}
		return ctrl.Result{}, nil
	}

	var argocdApp *unstructured.Unstructured
	var tenantApp *unstructured.Unstructured
	if req.Namespace == r.config.ArgoCD.Namespace {
		argocdApp = app
		ownerNs := argocdApp.GetLabels()[constants.OwnerAppNamespace]
		if len(ownerNs) == 0 {
			return ctrl.Result{}, nil
		}
		tenantApp = argocd.Application()
		err := r.client.Get(ctx, client.ObjectKey{Namespace: ownerNs, Name: argocdApp.GetName()}, tenantApp)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.Error(err, "Unable to find the corresponding tenant application.")
			}
			return ctrl.Result{}, err
		}
		if tenantApp.GetDeletionTimestamp() != nil {
			return ctrl.Result{}, nil
		}
		if argocdApp.GetDeletionTimestamp() != nil {
			logger.Info("argocd application is deleting. queue the tenant application", "tenantApp", tenantApp)
			r.channel <- event.GenericEvent{
				Object: tenantApp,
			}
			return ctrl.Result{}, nil
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
		if argocdApp != nil && argocdApp.GetDeletionTimestamp() != nil {
			logger.Info("argocd application is deleting. requeue the request")
			return ctrl.Result{
				Requeue: true,
			}, nil
		}
		if tenantApp.GetDeletionTimestamp() != nil {
			res, err := r.finalize(ctx, argocdApp, tenantApp)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to finalize: %w", err)
			}
			return res, nil
		}
	}

	return r.reconcileApplication(ctx, argocdApp, tenantApp)
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

func (r *ApplicationReconciler) reconcileApplication(ctx context.Context, argocdApp *unstructured.Unstructured, tenantApp *unstructured.Unstructured) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	canSync, err := r.canSyncApplicationSpec(ctx, argocdApp, tenantApp)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !canSync {
		logger.Info("cannot sync the application spec. requeue the request after 30 minutes")
		return ctrl.Result{
			RequeueAfter: 30 * time.Minute,
		}, nil
	}

	// Sync application spec from tenant to argocd.
	err = r.syncApplicationSpec(ctx, argocdApp, tenantApp)
	if err != nil {
		logger.Error(err, "failed to sync application spec")
		return ctrl.Result{}, err
	}
	// Sync application status from argocd to tenant.
	err = r.syncApplicationStatus(ctx, argocdApp, tenantApp)
	if err != nil {
		logger.Error(err, "failed to sync application status")
		return ctrl.Result{}, err
	}
	logger.Info("Application successfully reconciled")
	return ctrl.Result{}, nil
}

func (r *ApplicationReconciler) canSyncApplicationSpec(ctx context.Context, argocdApp *unstructured.Unstructured, tenantApp *unstructured.Unstructured) (bool, error) {
	logger := log.FromContext(ctx)
	ns := &corev1.Namespace{}
	err := r.client.Get(ctx, client.ObjectKey{Name: tenantApp.GetNamespace()}, ns)
	if err != nil {
		return false, err
	}
	tenantName, ok := ns.Labels[constants.OwnerTenant]
	if !ok {
		logger.Info("the namespace does not belong to a tenant", "targetNamespace", ns.Name)
		r.recorder.Eventf(tenantApp, corev1.EventTypeWarning, "CannotSync", "the namespace '%s' does not belong to a tenant", ns.Name)
		return false, nil
	}

	if argocdApp != nil {
		ownerNs, ok := argocdApp.GetLabels()[constants.OwnerAppNamespace]
		if !ok {
			project, found, err := unstructured.NestedString(argocdApp.UnstructuredContent(), "spec", "project")
			if err != nil {
				return false, err
			}
			if !found {
				return false, errors.New("spec.project not found in the application: " + argocdApp.GetNamespace() + "/" + argocdApp.GetName())
			}
			if project != tenantName {
				logger.Info("project of the application does not match the tenant name", "project", project, "tenantName", tenantName)
				r.recorder.Eventf(tenantApp, corev1.EventTypeWarning, "CannotSync", "project '%s' of the application '%s/%s' does not match the tenant name '%s'", project, argocdApp.GetNamespace(), argocdApp.GetName(), tenantName)
				return false, nil
			}
		} else if tenantApp.GetNamespace() != ownerNs {
			logger.Info("the application is already managed by another namespace", "tenantNamespace", tenantApp.GetNamespace(), "ownerNamespace", ownerNs)
			r.recorder.Eventf(tenantApp, corev1.EventTypeWarning, "CannotSync", "the application '%s/%s' is already managed by another namespace '%s'", tenantApp.GetNamespace(), tenantApp.GetName(), ownerNs)
			return false, nil
		}
	}

	project, found, err := unstructured.NestedString(tenantApp.UnstructuredContent(), "spec", "project")
	if err != nil {
		return false, err
	}
	if !found {
		return false, errors.New("spec.project not found in the application: " + tenantApp.GetNamespace() + "/" + tenantApp.GetName())
	}
	if project != tenantName {
		logger.Info("project of the application does not match the tenant name", "project", project, "tenantName", tenantName)
		r.recorder.Eventf(tenantApp, corev1.EventTypeWarning, "CannotSync", "project '%s' of the application '%s/%s' does not match the tenant name '%s'", project, tenantApp.GetNamespace(), tenantApp.GetName(), tenantName)
		return false, nil
	}

	return true, nil
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
	logger.Info("Sync application spec succeeded")
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

		// This is a workaround.
		// When `status.summary` is empty, controller will fail to sync status.
		// So, controller sets an empty slice as dummy data.
		summary, found, err := unstructured.NestedFieldCopy(newApp.UnstructuredContent(), "status", "summary")
		if err != nil {
			return err
		}
		if found {
			summaryMap, ok := summary.(map[string]interface{})
			if ok && len(summaryMap) == 0 {
				logger.Info("status.summary is empty")
				err = unstructured.SetNestedStringSlice(newApp.UnstructuredContent(), []string{""}, "status", "summary", "images")
				if err != nil {
					return err
				}
			}
		}
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
	logger.Info("Sync application status succeeded")
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
	src := source.Channel{
		Source: r.channel,
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
		Watches(&src, &handler.EnqueueRequestForObject{}).
		Complete(r)
}
