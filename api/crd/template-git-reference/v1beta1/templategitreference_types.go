/*
Copyright 2025 berops.com.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EndpointSpec defines the git endpoint configuration.
type EndpointSpec struct {
	// URL of the git repository.
	URL string `json:"url"`

	// Protocol used to access the repository.
	// +kubebuilder:validation:Enum=https;
	Protocol string `json:"protocol"`
}

// GitAuth defines optional authentication for the git repository.
type GitAuth struct {
	// SecretRef is a reference to a Kubernetes secret containing the git token.
	// +optional
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
}

// GitPaths defines paths to various template directories within the repository.
type GitPaths struct {
	// Terraformer path to terraformer templates.
	Terraformer string `json:"terraformer"`

	// Playbooks path to ansible playbooks.
	Playbooks string `json:"playbooks"`

	// ConfigLB path to loadbalancer configuration templates.
	ConfigLB string `json:"configLb"`

	// ConfigK8s path to kubernetes configuration templates.
	ConfigK8s string `json:"configK8s"`

	// ManifestsK8s path to kubernetes manifest templates.
	ManifestsK8s string `json:"manifestsK8s"`
}

// TemplateGitReferenceSpec defines the desired state of TemplateGitReference.
type TemplateGitReferenceSpec struct {
	// Endpoint of the git repository.
	Endpoint EndpointSpec `json:"endpoint"`

	// Auth defines optional authentication for the repository.
	// +optional
	Auth GitAuth `json:"auth,omitempty"`

	// Commit defines the commit, tag, or branch to checkout.
	Commit string `json:"commit"`

	// Paths defines the paths to template directories within the repository.
	Paths GitPaths `json:"paths"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.endpoint.url",description="URL of the git repository"
// +kubebuilder:printcolumn:name="Commit",type="string",JSONPath=".spec.commit",description="Commit/tag/branch reference"
// +kubebuilder:metadata:labels=app.kubernetes.io/part-of=claudie
// +kubebuilder:storageversion
// TemplateGitReference is a definition of a git repository reference containing templates.
// It specifies the endpoint, authentication, commit, and paths to various template directories.
type TemplateGitReference struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TemplateGitReferenceSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// TemplateGitReferenceList contains a list of TemplateGitReference.
type TemplateGitReferenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplateGitReference `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TemplateGitReference{}, &TemplateGitReferenceList{})
}
