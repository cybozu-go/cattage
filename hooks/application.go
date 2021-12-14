package hooks

import (
	"context"
	"net/http"

	"github.com/cybozu-go/neco-tenant-controller/pkg/argocd"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-argoproj-io-application,mutating=false,failurePolicy=fail,sideEffects=None,groups=argoproj.io,resources=applications,verbs=create;update,versions=v1alpha1,name=vapplication.kb.io,admissionReviewVersions={v1}

type applicationValidator struct {
	client.Client
	dec *admission.Decoder
}

var _ admission.Handler = &applicationValidator{}

func (v *applicationValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create {
		return admission.Allowed("")
	}
	app := argocd.Application()
	if err := v.dec.Decode(req, app); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	return admission.Allowed("")
}

// SetupApplicationWebhook registers the webhooks for Application
func SetupApplicationWebhook(mgr manager.Manager, dec *admission.Decoder) {
	serv := mgr.GetWebhookServer()

	v := &applicationValidator{
		Client: mgr.GetClient(),
		dec:    dec,
	}
	serv.Register("/validate-argoproj-io-application", &webhook.Admission{Handler: v})
}
