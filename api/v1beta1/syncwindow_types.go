package v1beta1

import (
	"github.com/cybozu-go/cattage/pkg/argocd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SyncWindowSpec defines the desired state of SyncWindow
type SyncWindowSpec struct {
	// SyncWindows is a list of sync windows
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	SyncWindows argocd.SyncWindows `json:"syncWindows"`
}

// SyncWindowStatus defines the observed state of SyncWindow
type SyncWindowStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions is an array of conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SyncWindow is the Schema for the syncwindows API
type SyncWindow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SyncWindowSpec   `json:"spec,omitempty"`
	Status SyncWindowStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SyncWindowList contains a list of SyncWindow
type SyncWindowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SyncWindow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SyncWindow{}, &SyncWindowList{})
}
