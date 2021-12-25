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

	tenantv1beta1 "github.com/cybozu-go/neco-tenant-controller/api/v1beta1"
	"github.com/cybozu-go/neco-tenant-controller/pkg/constants"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/mutate-multi-tenancy-cybozu-com-v1beta1-tenant,mutating=true,failurePolicy=fail,sideEffects=None,groups=multi-tenancy.cybozu.com,resources=tenants,verbs=create;update,versions=v1beta1,name=mtenant.kb.io,admissionReviewVersions={v1}

type tenantMutator struct {
	dec *admission.Decoder
}

var _ admission.Handler = &tenantMutator{}

func (m *tenantMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create {
		return admission.Allowed("")
	}

	sn := &tenantv1beta1.Tenant{}
	if err := m.dec.Decode(req, sn); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	sn.Finalizers = []string{constants.Finalizer}
	data, err := json.Marshal(sn)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, data)
}

//+kubebuilder:webhook:path=/validate-multi-tenancy-cybozu-com-v1beta1-tenant,mutating=false,failurePolicy=fail,sideEffects=None,groups=multi-tenancy.cybozu.com,resources=tenants,verbs=create;update,versions=v1beta1,name=vtenant.kb.io,admissionReviewVersions={v1}

type tenantValidator struct {
	client.Client
	dec *admission.Decoder
}

var _ admission.Handler = &tenantValidator{}

func (v *tenantValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create {
		return admission.Allowed("")
	}

	sn := &tenantv1beta1.Tenant{}
	if err := v.dec.Decode(req, sn); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	return admission.Allowed("")
}

// SetupTenantWebhook registers the webhooks for Tenant
func SetupTenantWebhook(mgr manager.Manager, dec *admission.Decoder) {
	serv := mgr.GetWebhookServer()

	m := &tenantMutator{
		dec: dec,
	}
	serv.Register("/mutate-multi-tenancy-cybozu-com-v1beta1-tenant", &webhook.Admission{Handler: m})

	v := &tenantValidator{
		Client: mgr.GetClient(),
		dec:    dec,
	}
	serv.Register("/validate-multi-tenancy-cybozu-com-v1beta1-tenant", &webhook.Admission{Handler: v})
}
