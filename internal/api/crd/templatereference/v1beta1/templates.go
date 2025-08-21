// Copyright 2025 berops.com.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:metadata:labels=app.kubernetes.io/part-of=claudie
//
// TemplateGitReference used for injecting templates used within Claudie to build kuberentes clusters.
type TemplateGitReference struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec Spec `json:"spec,omitzero"`
}

type Spec struct {
	// Endpoint specifies the URL to the git repository on which the templates to be downloaded
	// are being hosted.
	//
	// +kubebuilder:validation:Required
	Endpoint Endpoint `json:"endpoint"`

	// Authentication needed for accessing the git repository hosted at [Spec.Endpoint].
	//
	// +optional
	Auth Auth `json:"auth,omitempty"`

	// Commit to checkout to for the provided git repository [Spec.Endpoint]
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Commit string `json:"commit"`

	// Root directory of Claudie templates within the repository itself.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	RootDir string `json:"rootDir"`

	// Invididual sub paths starting from [Spec.RootDir] for different stages
	// for building the kuberentes cluster within Claudie.
	//
	// +kubebuilder:validation:Required
	SubPaths Templates `json:"subPaths"`
}

type Templates struct {
	// Path to the root directory of the templates to be used for spawning
	// the infrastructure in Terraform.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Terraformer string `json:"terraformer"`

	// Path to the root directory of the ansible-playbooks to be used after
	// the creation/update of the infrastructure to configure the invididual
	// nodes of the cluster.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Playbooks string `json:"playbooks"`

	// Path to the root directory of the envoy configuration to be used on the
	// loadbalancer nodes created by claudie.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	LBConfig string `json:"lbConfig"`

	// Path to the root directory of the kubernetes configuration that will be used
	// to create/reconcile the kuberenetes cluster on the provisioned infrastructure.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	K8sConfig string `json:"k8sConfig"`

	// Path to the root directory of the custom manifests to be applied on the created/reconciled
	// kuberentes cluster.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Manifests string `json:"manifests"`
}

type Auth struct {
	// Reference to the secret containing authentication for the git repository
	// under [Spec.Endpoint].
	//
	// +kubebuilder:validation:Required
	SecretRef corev1.SecretReference `json:"secretRef"`
}

type Endpoint struct {
	// The URL, without the protocol, for the templates.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// The protocol to be used with the provided URL.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Enum=https
	Protocol string `json:"protocol"`
}

// +kubebuilder:object:root=true
// TemplateGitReferenceList contains a list of TemplateGitReference
type TemplateGitReferenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TemplateGitReference `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TemplateGitReference{}, &TemplateGitReferenceList{})
}
