package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/cybozu-go/neco-tenant-controller/pkg/argocd"
	"github.com/cybozu-go/neco-tenant-controller/pkg/config"
	"github.com/cybozu-go/neco-tenant-controller/pkg/constants"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/mutate-argoproj-io-application,mutating=true,failurePolicy=fail,sideEffects=None,groups=argoproj.io,resources=applications,verbs=create;update,versions=v1alpha1,name=mapplication.kb.io,admissionReviewVersions={v1}

type applicationMutator struct {
	dec    *admission.Decoder
	config *config.Config
}

var _ admission.Handler = &applicationMutator{}

func (m *applicationMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create {
		return admission.Allowed("")
	}

	app := argocd.Application()
	if err := m.dec.Decode(req, app); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if app.GetNamespace() == m.config.ArgoCD.Namespace {
		return admission.Allowed("")
	}

	app.SetFinalizers([]string{constants.Finalizer})
	data, err := json.Marshal(app)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, data)
}

//+kubebuilder:webhook:path=/validate-argoproj-io-application,mutating=false,failurePolicy=fail,sideEffects=None,groups=argoproj.io,resources=applications,verbs=create;update,versions=v1alpha1,name=vapplication.kb.io,admissionReviewVersions={v1}

type applicationValidator struct {
	client.Client
	dec    *admission.Decoder
	config *config.Config
}

var _ admission.Handler = &applicationValidator{}

func (v *applicationValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation != admissionv1.Create && req.Operation != admissionv1.Update {
		return admission.Allowed("")
	}

	app := argocd.Application()
	if err := v.dec.Decode(req, app); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if app.GetNamespace() == v.config.ArgoCD.Namespace {
		return admission.Allowed("")
	}

	if app.GetDeletionTimestamp() != nil {
		return admission.Allowed("")
	}

	ns := &corev1.Namespace{}
	err := v.Client.Get(ctx, client.ObjectKey{Name: app.GetNamespace()}, ns)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	group, ok := ns.Labels[v.config.Namespace.GroupKey]
	if !ok {
		return admission.Denied("an application cannot be created on unmanaged namespaces")
	}

	apps := argocd.ApplicationList()
	err = v.Client.List(ctx, apps, client.InNamespace(v.config.ArgoCD.Namespace))
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	for _, a := range apps.Items {
		if app.GetName() == a.GetName() {
			ownerNs := a.GetLabels()[constants.OwnerAppNamespace]
			if ownerNs == "" {
				break
			}
			if app.GetNamespace() == ownerNs {
				break
			}
			return admission.Denied("cannot create an application with the same name")
		}
	}

	project, found, err := unstructured.NestedString(app.UnstructuredContent(), "spec", "project")
	if err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unable to get spec.project; %w", err))
	}
	if !found {
		return admission.Errored(http.StatusBadRequest, errors.New("spec.project not found"))
	}

	if group != project {
		return admission.Denied("cannot specify a project for other tenants")
	}

	return admission.Allowed("ok")
}

// SetupApplicationWebhook registers the webhooks for Application
func SetupApplicationWebhook(mgr manager.Manager, dec *admission.Decoder, config *config.Config) {
	serv := mgr.GetWebhookServer()

	m := &applicationMutator{
		dec:    dec,
		config: config,
	}
	serv.Register("/mutate-argoproj-io-application", &webhook.Admission{Handler: m})

	v := &applicationValidator{
		Client: mgr.GetClient(),
		dec:    dec,
		config: config,
	}
	serv.Register("/validate-argoproj-io-application", &webhook.Admission{Handler: v})
}
