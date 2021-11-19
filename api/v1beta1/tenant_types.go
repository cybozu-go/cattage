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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TenantSpec defines the desired state of Tenant
type TenantSpec struct {
	Namespaces []NamespaceSpec `json:"namespaces,omitempty"`
	ArgoCD     *ArgoCDSpec     `json:"argocd,omitempty"`
	Teleport   *TeleportSpec   `json:"teleport,omitempty"`
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
	// Applications are the list of Application resources managed by the tenant team.
	// +optional
	Applications []ArgoCDApplicationSpec `json:"applications,omitempty"`

	// Repositories are the list of repositories used by the tenant team.
	// +optional
	Repositories []string `json:"repositories,omitempty"`

	// ExtraAdmins are the names of the team to add to the AppProject user.
	// Specify this if you want other tenant teams to be able to use your AppProject.
	// +optional
	ExtraAdmins []string `json:"extraAdmins,omitempty"`
}

// ArgoCDApplicationSpec defines the desired state of Application
type ArgoCDApplicationSpec struct {
	// Name is the name of Application resource.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Path is a directory path within the Git repository, and is only valid for applications sourced from Git.
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// RepoURL is the URL to the repository (Git or Helm) that contains the application manifests.
	// +kubebuilder:validation:Required
	RepoURL string `json:"repoURL"`

	// TargetRevision defines the revision of the source to sync the application to.
	// In case of Git, this can be commit, tag, or branch. If omitted, will equal to HEAD.
	// In case of Helm, this is a semver tag for the Chart's version.
	// +kubebuilder:validation:Required
	TargetRevision string `json:"targetRevision"`
}

// TeleportSpec defines the desired state of the settings for Teleport
type TeleportSpec struct {
	// Node is the settings of Teleport Node for the tenant team.
	// +optional
	Node *TeleportNodeSpec `json:"node,omitempty"`

	// Applications are the list of applications to be used by the tenant team.
	// +optional
	Applications []TeleportApplicationSpec `json:"applications,omitempty"`
}

type TeleportNodeSpec struct {
	// Replicas is the number of Teleport Node Pods.
	// +kubebuilder:validation:Required
	Replicas int `json:"replicas"`

	// ExtraArgs are the list of additional arguments to be specified for Teleport Node Pod.
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// TeleportApplicationSpec defines the desired state of Teleport Application.
type TeleportApplicationSpec struct {
	// Name is the name of the application to proxy.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// URL is the internal address of the application to proxy.
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// ExtraArgs are the list of additional arguments to be specified for Teleport Application Pod.
	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// TenantStatus defines the observed state of Tenant
type TenantStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
