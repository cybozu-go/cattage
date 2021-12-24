package config

import (
	"errors"
	v1labelvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

// Config represents the configuration file of neco-tenant-controller.
type Config struct {
	Namespace NamespaceConfig `json:"namespace,omitempty"`
	ArgoCD    ArgoCDConfig    `json:"argocd,omitempty"`
}

// NamespaceConfig represents the configuration about Namespaces
type NamespaceConfig struct {
	// CommonLabels are labels to add to all namespaces to be deployed by neco-tenant-controller
	CommonLabels map[string]string `json:"commonLabels,omitempty"`

	GroupKey string `json:"groupKey"`

	RoleBindingTemplate string `json:"rolebindingTemplate"`
}

// ArgoCDConfig represents the configuration about Argo CD
type ArgoCDConfig struct {
	// Namespace is the name of namespace where Argo CD is deployed
	Namespace string `json:"namespace"`

	AppProjectTemplate string `json:"appProjectTemplate"`
}

// Validate validates the configurations.
func (c *Config) Validate() error {

	var allErrs field.ErrorList
	allErrs = append(allErrs, v1labelvalidation.ValidateLabels(c.Namespace.CommonLabels, field.NewPath("namespace", "commonLabels"))...)

	allErrs = append(allErrs, v1labelvalidation.ValidateLabelName(c.Namespace.GroupKey, field.NewPath("namespace", "groupKey"))...)

	if len(c.Namespace.RoleBindingTemplate) == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("namespace", "rolebindingTemplate"), c.Namespace.RoleBindingTemplate, "should not be empty"))
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
