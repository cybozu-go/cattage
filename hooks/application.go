package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/cybozu-go/cattage/pkg/accurate"
	"github.com/cybozu-go/cattage/pkg/argocd"
	"github.com/cybozu-go/cattage/pkg/config"
	"github.com/cybozu-go/cattage/pkg/constants"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
	// An application created with argocd cli will have an empty namespace.
	if app.GetNamespace() == "" || app.GetNamespace() == m.config.ArgoCD.Namespace {
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

	tenantApp := argocd.Application()
	if err := v.dec.Decode(req, tenantApp); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	// An application created with argocd cli will have an empty namespace.
	if tenantApp.GetNamespace() == "" || tenantApp.GetNamespace() == v.config.ArgoCD.Namespace {
		return admission.Allowed("")
	}

	if tenantApp.GetDeletionTimestamp() != nil {
		return admission.Allowed("")
	}

	ns := &corev1.Namespace{}
	err := v.Client.Get(ctx, client.ObjectKey{Name: tenantApp.GetNamespace()}, ns)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	tenantName, ok := ns.Labels[constants.OwnerTenant]
	if !ok {
		return admission.Denied("cannot create the application on a namespace that does not belong to a tenant")
	}

	argocdApp := argocd.Application()
	err = v.Client.Get(ctx, client.ObjectKey{Namespace: v.config.ArgoCD.Namespace, Name: tenantApp.GetName()}, argocdApp)
	if err != nil && !apierrors.IsNotFound(err) {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if !apierrors.IsNotFound(err) {
		ownerNs, ok := argocdApp.GetLabels()[constants.OwnerAppNamespace]
		if !ok {
			project, found, err := unstructured.NestedString(argocdApp.UnstructuredContent(), "spec", "project")
			if err != nil {
				return admission.Errored(http.StatusBadRequest, fmt.Errorf("unable to get spec.project; %w", err))
			}
			if !found {
				return admission.Errored(http.StatusBadRequest, errors.New("spec.project not found"))
			}
			if project != tenantName {
				return admission.Denied(field.Forbidden(field.NewPath("spec", "project"), "project of the application does not match the tenant name").Error())
			}
		} else if tenantApp.GetNamespace() != ownerNs {
			return admission.Denied(field.Forbidden(field.NewPath("metadata", "namespace"), "the application is already managed by another namespace").Error())
		}
	}

	project, found, err := unstructured.NestedString(tenantApp.UnstructuredContent(), "spec", "project")
	if err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("unable to get spec.project; %w", err))
	}
	if !found {
		return admission.Errored(http.StatusBadRequest, errors.New("spec.project not found"))
	}

	if tenantName != project {
		return admission.Denied(field.Forbidden(field.NewPath("spec", "project"), "project of the application does not match the tenant name").Error())
	}

	nsType := ns.Labels[accurate.LabelType]
	if nsType == accurate.NSTypeRoot {
		return admission.Allowed("").WithWarnings(
			"The application resource has been created on a root namespace.",
			"It is recommended to create the application resource on a sub-namespace.")
	}

	return admission.Allowed("")
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
