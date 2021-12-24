package config

import (
	"k8s.io/apimachinery/pkg/api/meta"
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
func (c *Config) Validate(mapper meta.RESTMapper) error {
	return nil
}

// Load loads configurations.
func (c *Config) Load(data []byte) error {
	return yaml.Unmarshal(data, c, yaml.DisallowUnknownFields)
}
