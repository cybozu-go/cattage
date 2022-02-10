package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TenantSpec defines the desired state of Tenant.
type TenantSpec struct {
	// RootNamespaces are the list of root namespaces that belong to this tenant.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	RootNamespaces []RootNamespaceSpec `json:"rootNamespaces"`

	// ArgoCD is the settings of Argo CD for this tenant.
	// +optional
	ArgoCD ArgoCDSpec `json:"argocd,omitempty"`

	// Delegates is a list of other tenants that are delegated access to this tenant.
	// +optional
	Delegates []DelegateSpec `json:"delegates,omitempty"`
}

// RootNamespaceSpec defines the desired state of Namespace.
type RootNamespaceSpec struct {
	// Name is the name of namespace to be generated.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Labels are the labels to add to the namespace.
	// This supersedes `namespace.commonLabels` in the configuration.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are the annotations to add to the namespace.
	// This supersedes `namespace.commonAnnotations` in the configuration.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ArgoCDSpec defines the desired state of the settings for Argo CD.
type ArgoCDSpec struct {
	// Repositories contains list of repository URLs which can be used by the tenant.
	// +optional
	Repositories []string `json:"repositories,omitempty"`
}

// DelegateSpec defines a tenant that is delegated access to a tenant.
type DelegateSpec struct {
	// Name is the name of a delegated tenant.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Roles is a list of roles that the tenant has.
	// +kubebuilder:validation:MinItems=1
	Roles []string `json:"roles"`
}

// TenantHealth defines the observed state of Tenant.
// +kubebuilder:validation:Enum=Healthy;Unhealthy
type TenantHealth string

const (
	TenantHealthy   = TenantHealth("Healthy")
	TenantUnhealthy = TenantHealth("Unhealthy")
)

// TenantStatus defines the observed state of Tenant.
type TenantStatus struct {
	// Health is the health of Tenant.
	// +optional
	Health TenantHealth `json:"health,omitempty"`

	// Conditions is an array of conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

const (
	ConditionReady string = "Ready"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".status.health"

// Tenant is the Schema for the tenants API.
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TenantList contains a list of Tenant.
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}
