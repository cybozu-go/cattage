/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	multitenancyv1beta1 "github.com/cybozu-go/neco-tenant-controller/api/v1beta1"
	"github.com/cybozu-go/neco-tenant-controller/pkg/argocd"
	"github.com/cybozu-go/neco-tenant-controller/pkg/config"
	"github.com/cybozu-go/neco-tenant-controller/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// TenantReconciler reconciles a Tenant object
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.Config
}

//+kubebuilder:rbac:groups=multi-tenancy.cybozu.com,resources=tenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=multi-tenancy.cybozu.com,resources=tenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=multi-tenancy.cybozu.com,resources=tenants/finalizers,verbs=update
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
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	tenant := &multitenancyv1beta1.Tenant{}
	if err := r.Get(ctx, req.NamespacedName, tenant); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if tenant.DeletionTimestamp != nil {
		logger.Info("starting finalization")
		if err := r.finalize(ctx, tenant); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to finalize: %w", err)
		}
		logger.Info("finished finalization")
		return ctrl.Result{}, nil
	}

	err := r.reconcileNamespaces(ctx, tenant)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.reconcileArgoCD(ctx, tenant)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func containNamespace(roots []multitenancyv1beta1.NamespaceSpec, ns corev1.Namespace) bool {
	for _, root := range roots {
		if root.Name == ns.Name {
			return true
		}
	}
	return false
}

func (r *TenantReconciler) removeManagedLabels(ctx context.Context, tenant *multitenancyv1beta1.Tenant, orphan bool) error {
	logger := log.FromContext(ctx)
	nss := &corev1.NamespaceList{}
	if err := r.List(ctx, nss, client.MatchingFields{constants.NamespaceGroupKey: tenant.Name}); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}
	for _, ns := range nss.Items {
		if orphan && containNamespace(tenant.Spec.Namespaces, ns) {
			continue
		}
		logger.Info("Remove labels", "ns", ns)
		newNs := ns.DeepCopy()
		delete(newNs.Labels, constants.OwnerTenant)
		delete(newNs.Labels, r.Config.Namespace.GroupKey)
		patch := client.MergeFrom(&ns)
		err := r.Patch(ctx, newNs, patch)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *TenantReconciler) finalize(ctx context.Context, tenant *multitenancyv1beta1.Tenant) error {
	if !controllerutil.ContainsFinalizer(tenant, constants.Finalizer) {
		return nil
	}

	err := r.removeManagedLabels(ctx, tenant, false)
	if err != nil {
		return err
	}

	controllerutil.RemoveFinalizer(tenant, constants.Finalizer)
	return r.Update(ctx, tenant)
}

func (r *TenantReconciler) reconcileNamespaces(ctx context.Context, tenant *multitenancyv1beta1.Tenant) error {
	logger := log.FromContext(ctx)
	for _, ns := range tenant.Spec.Namespaces {
		obj := &corev1.Namespace{}
		obj.Name = ns.Name
		op, err := ctrl.CreateOrUpdate(ctx, r.Client, obj, func() error {
			if len(obj.Labels) == 0 {
				obj.Labels = map[string]string{}
			}
			for k, v := range r.Config.Namespace.CommonLabels {
				obj.Labels[k] = v
			}
			for k, v := range ns.Labels {
				obj.Labels[k] = v
			}
			for k, v := range ns.Annotations {
				obj.Annotations[k] = v
			}
			obj.Labels["accurate.cybozu.com/type"] = "root"
			obj.Labels[r.Config.Namespace.GroupKey] = tenant.Name
			obj.Labels[constants.OwnerTenant] = tenant.Name

			return nil
		})
		if err != nil {
			return err
		}

		rb := &rbacv1.RoleBinding{}
		rb.SetNamespace(ns.Name)
		rb.SetName(tenant.Name + "-admin")

		op, err = ctrl.CreateOrUpdate(ctx, r.Client, rb, func() error {
			if rb.Labels == nil {
				rb.Labels = map[string]string{}
			}
			rb.Labels[constants.OwnerTenant] = tenant.Name

			if rb.Annotations == nil {
				rb.Annotations = map[string]string{}
			}
			rb.Annotations["accurate.cybozu.com/propagate"] = "update"
			rb.RoleRef.Name = "admin"
			rb.RoleRef.Kind = "ClusterRole"
			rb.RoleRef.APIGroup = "rbac.authorization.k8s.io"

			rb.Subjects = []rbacv1.Subject{}
			rb.Subjects = append(rb.Subjects, rbacv1.Subject{
				Kind:     "Group",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     tenant.Name,
			})
			rb.Subjects = append(rb.Subjects, rbacv1.Subject{ //TODO:
				Kind:      "ServiceAccount",
				Namespace: r.Config.Teleport.Namespace,
				Name:      "node-" + tenant.Name,
			})

			for _, admin := range ns.ExtraAdmins {
				rb.Subjects = append(rb.Subjects, rbacv1.Subject{
					Kind:     "Group",
					APIGroup: "rbac.authorization.k8s.io",
					Name:     admin,
				})
				rb.Subjects = append(rb.Subjects, rbacv1.Subject{
					Kind:      "ServiceAccount",
					Namespace: r.Config.Teleport.Namespace,
					Name:      "node-" + admin,
				})
			}
			return nil
		})
		if err != nil {
			logger.Error(err, "failed to upsert RoleBinding")
			return err
		}
		logger.Info("updated rolebinding", "op", op)
	}
	// Remove orphan labels
	err := r.removeManagedLabels(ctx, tenant, true)
	if err != nil {
		return err
	}

	return nil
}

func (r *TenantReconciler) reconcileArgoCD(ctx context.Context, tenant *multitenancyv1beta1.Tenant) error {
	logger := log.FromContext(ctx)

	proj := argocd.AppProject()

	err := r.Get(ctx, client.ObjectKey{Namespace: r.Config.ArgoCD.Namespace, Name: tenant.Name}, proj)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	dec := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err = dec.Decode([]byte(r.Config.ArgoCD.AppProjectTemplate), nil, proj)
	if err != nil {
		return err
	}

	nss := &corev1.NamespaceList{}
	if err := r.List(ctx, nss, client.MatchingFields{constants.NamespaceGroupKey: tenant.Name}); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	proj.SetNamespace(r.Config.ArgoCD.Namespace)
	proj.SetName(tenant.Name)
	proj.SetLabels(map[string]string{
		constants.OwnerTenant: tenant.Name,
	})

	spec := proj.UnstructuredContent()["spec"].(map[string]interface{})

	destinations, ok := spec["destinations"].([]map[string]interface{})
	if !ok {
		destinations = []map[string]interface{}{}
	}
	for _, ns := range nss.Items {
		destinations = append(destinations, map[string]interface{}{
			"namespace": ns.Name,
			"server":    "*",
		})
	}

	groups := []string{
		fmt.Sprintf("%s:%s", r.Config.ArgoCD.Organization, tenant.Name),
	}
	if tenant.Spec.ArgoCD != nil {
		for _, extra := range tenant.Spec.ArgoCD.ExtraAdmins {
			groups = append(groups, fmt.Sprintf("%s:%s", r.Config.ArgoCD.Organization, extra))
		}
	}

	roles := []map[string]interface{}{
		{
			"groups": groups,
			"name":   "admin",
			"policies": []string{
				fmt.Sprintf("p, proj:%s:admin, applications, *, %s/*, allow", tenant.Name, tenant.Name),
			},
		},
	}

	spec["destinations"] = destinations
	spec["roles"] = roles
	proj.UnstructuredContent()["spec"] = spec

	err = r.Patch(ctx, proj, client.Apply, &client.PatchOptions{
		Force:        pointer.BoolPtr(true),
		FieldManager: constants.FieldManager,
	})

	if err != nil {
		return err
	}

	logger.Info("AppProject successfully reconciled")

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	tenantHandler := func(o client.Object, q workqueue.RateLimitingInterface) {
		owner := o.GetLabels()[constants.OwnerTenant]
		if owner == "" {
			return
		}
		q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
			Name: owner,
		}})
	}

	funcs := handler.Funcs{
		CreateFunc: func(ev event.CreateEvent, q workqueue.RateLimitingInterface) {
			tenantHandler(ev.Object, q)
		},
		UpdateFunc: func(ev event.UpdateEvent, q workqueue.RateLimitingInterface) {
			if ev.ObjectNew.GetDeletionTimestamp() != nil {
				return
			}
			tenantHandler(ev.ObjectOld, q)
		},
		DeleteFunc: func(ev event.DeleteEvent, q workqueue.RateLimitingInterface) {
			tenantHandler(ev.Object, q)
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&multitenancyv1beta1.Tenant{}).
		Watches(&source.Kind{Type: &corev1.Namespace{}}, funcs).
		Watches(&source.Kind{Type: &rbacv1.RoleBinding{}}, funcs).
		Watches(&source.Kind{Type: argocd.AppProject()}, funcs).
		Complete(r)
}

func SetupIndexForNamespace(ctx context.Context, mgr manager.Manager, groupKey string) error {
	ns := &corev1.Namespace{}
	return mgr.GetFieldIndexer().IndexField(ctx, ns, constants.NamespaceGroupKey, func(rawObj client.Object) []string {
		nsType := rawObj.GetLabels()["accurate.cybozu.com/type"]
		if nsType != "root" {
			return nil
		}
		group := rawObj.GetLabels()[groupKey]
		if group == "" {
			return nil
		}
		return []string{group}
	})
}
