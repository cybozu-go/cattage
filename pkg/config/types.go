package config

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/yaml"
)

// Config represents the configuration file of neco-tenant-controller.
type Config struct {
	Namespace NamespaceConfig `json:"namespace,omitempty"`
	ArgoCD    ArgoCDConfig    `json:"argocd,omitempty"`
	Teleport  TeleportConfig  `json:"teleport,omitempty"`
}

// NamespaceConfig represents the configuration about Namespaces
type NamespaceConfig struct {
	// CommonLabels are labels to add to all namespaces to be deployed by neco-tenant-controller
	CommonLabels map[string]string `json:"commonLabels,omitempty"`
}

// ArgoCDConfig represents the configuration about Argo CD
type ArgoCDConfig struct {
	// Namespace is the name of namespace where Argo CD is deployed
	Namespace string `json:"namespace"`
	// PermissiveValidation is the mode of validation for Application resources.
	// If true is set, this does not deny Application resources but issues a warning.
	PermissiveValidation bool `json:"permissiveValidation"`
}

// TeleportConfig represents the configuration about Teleport
type TeleportConfig struct {
	// Namespace is the name of namespace where Teleport Nodes are deployed
	Namespace string `json:"namespace"`
	// Image is the name of Teleport container image
	Image string `json:"image"`
	// LicenseSecretName is the name of secret resource contains a license key for Teleport Enterprise
	LicenseSecretName string `json:"licenseSecretName"`
}

// Validate validates the configurations.
func (c *Config) Validate(mapper meta.RESTMapper) error {
	return nil
}

// Load loads configurations.
func (c *Config) Load(data []byte) error {
	return yaml.Unmarshal(data, c, yaml.DisallowUnknownFields)
}
