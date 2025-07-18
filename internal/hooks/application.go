package hooks

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cybozu-go/cattage/internal/argocd"
	"github.com/cybozu-go/cattage/internal/config"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-argoproj-io-application,mutating=false,failurePolicy=fail,sideEffects=None,groups=argoproj.io,resources=applications,verbs=create;update,versions=v1alpha1,name=vapplication.kb.io,admissionReviewVersions={v1}

type applicationValidator struct {
	client client.Client
	dec    admission.Decoder
	config *config.Config
}

var _ admission.Handler = &applicationValidator{}

func (v *applicationValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if !v.config.ArgoCD.PreventAppCreationInArgoCDNamespace {
		return admission.Allowed("")
	}

	app := argocd.Application()
	if err := v.dec.Decode(req, app); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if app.GetNamespace() == v.config.ArgoCD.Namespace {
		if req.Operation != admissionv1.Create {
			return admission.Allowed("").WithWarnings(fmt.Sprintf("creating Application in %s namespace is forbidden", v.config.ArgoCD.Namespace))
		}
		return admission.Denied(fmt.Sprintf("cannot create Application in %s namespace", v.config.ArgoCD.Namespace))
	}

	return admission.Allowed("")
}

// SetupApplicationWebhook registers the webhooks for Application
func SetupApplicationWebhook(mgr manager.Manager, dec admission.Decoder, config *config.Config) {
	serv := mgr.GetWebhookServer()

	v := &applicationValidator{
		client: mgr.GetClient(),
		dec:    dec,
		config: config,
	}
	serv.Register("/validate-argoproj-io-application", &webhook.Admission{Handler: v})
}
