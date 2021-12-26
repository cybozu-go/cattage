/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TenantSpec defines the desired state of Tenant
type TenantSpec struct {
	Namespaces []NamespaceSpec `json:"namespaces,omitempty"`
	ArgoCD     ArgoCDSpec      `json:"argocd,omitempty"`
}

// NamespaceSpec defines the desired state of Namespace
type NamespaceSpec struct {
	// Name is the name of namespace to be generated
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Labels are the labels to add to the namespace
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are the annotations to add to the namespace
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// ExtraAdmins are the names of the team to add to the namespace administrator.
	// Specify this if you want other tenant teams to be able to use your namespace.
	// +optional
	ExtraAdmins []string `json:"extraAdmins,omitempty"`
}

// ArgoCDSpec defines the desired state of the settings for Argo CD
type ArgoCDSpec struct {
	// ExtraAdmins are the names of the team to add to the AppProject user.
	// Specify this if you want other tenant teams to be able to use your AppProject.
	// +optional
	ExtraAdmins []string `json:"extraAdmins,omitempty"`
}

// TenantStatus defines the observed state of Tenant
type TenantStatus struct {
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
//+kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"

// Tenant is the Schema for the tenants API
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TenantList contains a list of Tenant
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}
