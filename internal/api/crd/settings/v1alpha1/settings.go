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
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:metadata:labels=app.kubernetes.io/part-of=claudie
//
// Settings used for customization of deployed clusters via the InputManifest.
type Setting struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec Spec `json:"spec,omitzero"`
}

type Spec struct {
	// Envoy configuration to be referenced by a Role
	// in a LoadBalancer cluster in the InputManifest.
	//
	// +optional
	Envoy Envoy `json:"envoy,omitzero"`
}

type Envoy struct {
	// Specifies the dynamic listener configuration that will replace the
	// default configuration provided by claudie.
	//
	// Be careful when replacing the default configuration as you may break
	// the 'settings' configurable options for the role definition in the
	// InputManifest.
	//
	// If you need to change the default behaviour, it is advisable to start
	// with the default configuration provided by claudie, which matches the
	// configurable options in the InputManifest, and then make your own changes
	// from there.
	//
	// +optional
	Lds string `json:"lds,omitempty"`

	// Specifies the cluster dynamic configuration which will replace
	// the default claudie provided configuration.
	//
	// Be careful when replacing the default configuration as you may break
	// the 'settings' configurable options for the role definition in the
	// InputManifest.
	//
	// If you need to change the default behaviour, it is advisable to start
	// with the default configuration provided by claudie, which matches the
	// configurable options in the InputManifest, and then make your own changes
	// from there.
	//
	// +optional
	Cds string `json:"cds,omitempty"`
}

// +kubebuilder:object:root=true
// SettingsList contains a list of settings
type SettingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Setting `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Setting{}, &SettingList{})
}
