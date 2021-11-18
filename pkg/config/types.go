package config

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/yaml"
)

// Config represents the configuration file of neco-tenant-controller.
type Config struct {
	ArgoCDNamespace   string `json:"argocdNamespace,omitempty"`
	TeleportNamespace string `json:"teleportNamespace,omitempty"`
}

// Validate validates the configurations.
func (c *Config) Validate(mapper meta.RESTMapper) error {
	return nil
}

// Load loads configurations.
func (c *Config) Load(data []byte) error {
	return yaml.Unmarshal(data, c, yaml.DisallowUnknownFields)
}
