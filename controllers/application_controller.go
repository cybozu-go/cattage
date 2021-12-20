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
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/typed"
)

// ApplicationReconciler reconciles an Application object
type ApplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.Config
}

//+kubebuilder:rbac:groups=argoproj.io,resources=applications,verbs=get;list;watch;create;update;patch;delete

func (r *ApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	app := argocd.Application()
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var argocdApp *unstructured.Unstructured
	var tenantApp *unstructured.Unstructured
	if req.Namespace == r.Config.ArgoCD.Namespace {
		if app.GetDeletionTimestamp() != nil {
			return ctrl.Result{}, nil
		}
		argocdApp = app
		ownerNs := argocdApp.GetLabels()[constants.OwnerAppNamespace]
		if len(ownerNs) == 0 {
			return ctrl.Result{}, nil
		}
		ownerName := argocdApp.GetLabels()[constants.OwnerApplication]
		if len(ownerName) == 0 {
			return ctrl.Result{}, nil
		}
		tenantApp = argocd.Application()
		err := r.Get(ctx, client.ObjectKey{Namespace: ownerNs, Name: ownerName}, tenantApp)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		tenantApp = app
		argocdApp = argocd.Application()
		err := r.Get(ctx, client.ObjectKey{Namespace: r.Config.ArgoCD.Namespace, Name: tenantApp.GetName()}, argocdApp)
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
		err := r.Update(ctx, tenantApp)
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
	err := r.Delete(ctx, argocdApp)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *ApplicationReconciler) reconcileApplication(ctx context.Context, argocdApp *unstructured.Unstructured, tenantApp *unstructured.Unstructured) error {
	logger := log.FromContext(ctx)
	err := r.syncApplicationSpec(ctx, argocdApp, tenantApp)
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
	m["metadata"].(map[string]interface{})["namespace"] = r.Config.ArgoCD.Namespace
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
	labels[constants.OwnerApplication] = tenantApp.GetName()
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
	newApp.SetNamespace(r.Config.ArgoCD.Namespace)
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

	return r.Patch(ctx, newApp, client.Apply, &client.PatchOptions{
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
	return r.Patch(ctx, newApp, client.Apply, &client.PatchOptions{
		Force:        pointer.BoolPtr(true),
		FieldManager: constants.FieldManager,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(argocd.Application()).
		Complete(r)
}
