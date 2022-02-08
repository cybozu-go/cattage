package config

import (
	"errors"

	v1annotationvalidation "k8s.io/apimachinery/pkg/api/validation"
	v1labelvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

// Config represents the configuration file of cattage.
type Config struct {
	Namespace NamespaceConfig `json:"namespace,omitempty"`
	ArgoCD    ArgoCDConfig    `json:"argocd,omitempty"`
}

// NamespaceConfig represents the configuration about Namespaces
type NamespaceConfig struct {
	// CommonLabels are labels to be added to all namespaces belonging to a tenant
	// This may be overridden by `rootNamespaces.labels` of a tenant resource.
	CommonLabels map[string]string `json:"commonLabels,omitempty"`

	// CommonAnnotations are annotations to be added to all namespaces belonging to a tenant
	// This may be overridden by `rootNamespaces.annotations` of a tenant resource.
	CommonAnnotations map[string]string `json:"commonAnnotations,omitempty"`

	// RoleBindingTemplate is a template for RoleBinding resource that is created on all namespaces belonging to a tenant
	RoleBindingTemplate string `json:"roleBindingTemplate"`
}

// ArgoCDConfig represents the configuration about Argo CD
type ArgoCDConfig struct {
	// Namespace is the name of namespace where Argo CD is running
	Namespace string `json:"namespace"`

	// AppProjectTemplate is a template for AppProject resources that is created for each tenant
	AppProjectTemplate string `json:"appProjectTemplate"`
}

// Validate validates the configurations.
func (c *Config) Validate() error {

	var allErrs field.ErrorList
	allErrs = append(allErrs, v1labelvalidation.ValidateLabels(c.Namespace.CommonLabels, field.NewPath("namespace", "commonLabels"))...)
	allErrs = append(allErrs, v1annotationvalidation.ValidateAnnotations(c.Namespace.CommonAnnotations, field.NewPath("namespace", "commonAnnotations"))...)

	if len(c.Namespace.RoleBindingTemplate) == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("namespace", "roleBindingTemplate"), c.Namespace.RoleBindingTemplate, "should not be empty"))
	}

	for _, msg := range validation.IsDNS1123Subdomain(c.ArgoCD.Namespace) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("argocd", "namespace"), c.ArgoCD.Namespace, msg))
	}
	if len(c.ArgoCD.AppProjectTemplate) == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("argocd", "appProjectTemplate"), c.ArgoCD.AppProjectTemplate, "should not be empty"))
	}

	if len(allErrs) != 0 {
		return errors.New(allErrs.ToAggregate().Error())
	}

	return nil
}

// Load loads configurations.
func (c *Config) Load(data []byte) error {
	return yaml.Unmarshal(data, c, yaml.DisallowUnknownFields)
}
