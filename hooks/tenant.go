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

package hooks

import (
	"context"
	"encoding/json"
	"net/http"

	cattagev1beta1 "github.com/cybozu-go/cattage/api/v1beta1"
	"github.com/cybozu-go/cattage/pkg/accurate"
	"github.com/cybozu-go/cattage/pkg/config"
	"github.com/cybozu-go/cattage/pkg/constants"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/mutate-multi-tenancy-cybozu-com-v1beta1-tenant,mutating=true,failurePolicy=fail,sideEffects=None,groups=cattage.cybozu.io,resources=tenants,verbs=create;update,versions=v1beta1,name=mtenant.kb.io,admissionReviewVersions={v1}

type tenantMutator struct {
	dec *admission.Decoder
}

var _ admission.Handler = &tenantMutator{}

func (m *tenantMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create {
		return admission.Allowed("")
	}

	tenant := &cattagev1beta1.Tenant{}
	if err := m.dec.Decode(req, tenant); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	tenant.Finalizers = []string{constants.Finalizer}
	data, err := json.Marshal(tenant)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, data)
}

//+kubebuilder:webhook:path=/validate-multi-tenancy-cybozu-com-v1beta1-tenant,mutating=false,failurePolicy=fail,sideEffects=None,groups=cattage.cybozu.io,resources=tenants,verbs=create;update,versions=v1beta1,name=vtenant.kb.io,admissionReviewVersions={v1}

type tenantValidator struct {
	client client.Client
	dec    *admission.Decoder
	config *config.Config
}

var _ admission.Handler = &tenantValidator{}

func (v *tenantValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create && req.Operation != admissionv1.Update {
		return admission.Allowed("")
	}

	tenant := &cattagev1beta1.Tenant{}
	if err := v.dec.Decode(req, tenant); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	for _, ns := range tenant.Spec.Namespaces {
		namespace := &corev1.Namespace{}
		err := v.client.Get(ctx, client.ObjectKey{Name: ns.Name}, namespace)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		owner := namespace.Labels[constants.OwnerTenant]
		if owner != "" && owner != tenant.Name {
			return admission.Denied("deny to specify other owner's namespace")
		}
		nsType := namespace.Labels[accurate.LabelType]
		if nsType != "" && nsType != accurate.NSTypeRoot {
			return admission.Denied("deny to specify a namespace other than root")
		}
		parent := namespace.Labels[accurate.LabelParent]
		if parent != "" {
			return admission.Denied("deny to specify a sub namespace")
		}
	}

	return admission.Allowed("")
}

// SetupTenantWebhook registers the webhooks for Tenant
func SetupTenantWebhook(mgr manager.Manager, dec *admission.Decoder, config *config.Config) {
	serv := mgr.GetWebhookServer()

	m := &tenantMutator{
		dec: dec,
	}
	serv.Register("/mutate-multi-tenancy-cybozu-com-v1beta1-tenant", &webhook.Admission{Handler: m})

	v := &tenantValidator{
		client: mgr.GetClient(),
		dec:    dec,
		config: config,
	}
	serv.Register("/validate-multi-tenancy-cybozu-com-v1beta1-tenant", &webhook.Admission{Handler: v})
}
