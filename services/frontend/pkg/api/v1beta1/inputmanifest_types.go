/*
Copyright 2023 berops.com.

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
	"fmt"

	"github.com/berops/claudie/internal/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Provider defines the desired state of Provider
type Provider struct {
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:MinLength=1
	ProviderName string `json:"name"`
	// +kubebuilder:validation:Enum=gcp;hetzner;aws;oci;azure;cloudflare;hetznerdns;
	ProviderType string                 `json:"providerType"`
	SecretRef    corev1.SecretReference `json:"secretRef"`
}

// a simple enum type of providers
type ProviderType string

const (
	AWS         ProviderType = "aws"
	AZURE       ProviderType = "azure"
	CLOUDFLARE  ProviderType = "cloudflare"
	GCP         ProviderType = "gcp"
	HETZNER     ProviderType = "hetzner"
	HETZNER_DNS ProviderType = "hetznerdns"
	OCI         ProviderType = "oci"
)

type SecretField string

const (
	AWS_ACCESS_KEY          SecretField = "accesskey"
	AWS_SECRET_KEY          SecretField = "secretkey"
	AZURE_CLIENT_SECRET     SecretField = "clientsecret"
	AZURE_SUBSCRIPTION_ID   SecretField = "subscriptionid"
	AZURE_TENANT_ID         SecretField = "tenantid"
	AZURE_CLIENT_ID         SecretField = "clientid"
	CF_API_TOKEN            SecretField = "apitoken"
	GCP_CREDENTIALS         SecretField = "credentials"
	GCP_GCP_PROJECT         SecretField = "gcpproject"
	HETZNER_API_TOKEN       SecretField = "apitoken"
	HETZNER_DNS_CREDENTIALS SecretField = "credentials"
	OCI_PRIVATE_KEY         SecretField = "privatekey"
	OCI_KEY_FINGERPRINT     SecretField = "keyfingerprint"
	OCI_TENANCT_OCID        SecretField = "tenancyocid"
	OCI_USER_OCID           SecretField = "userocid"
	OCI_COMPARTMENT_OCID    SecretField = "compartmentocid"
)

// ProviderWithData helper type that help in conversion
type ProviderWithData struct {
	ProviderName string
	ProviderType ProviderType
	Secret       corev1.Secret
}

// InputManifestSpec defines the desired state of InputManifest
type InputManifestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make crds" to regenerate code after modifying this file

	// +optional
	Providers    []Provider            `json:"providers,omitempty"`
	// +optional
	NodePools    manifest.NodePool     `json:"nodePools,omitempty"`
	// +optional
	Kubernetes   manifest.Kubernetes   `json:"kubernetes,omitempty"`
	// +optional
	LoadBalancer manifest.LoadBalancer `json:"loadBalancers,omitempty"`
}

// InputManifestStatus defines the observed state of InputManifest
type InputManifestStatus struct {
	State   string `json:"state,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// InputManifest is the Schema for the inputmanifests API
type InputManifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InputManifestSpec   `json:"spec,omitempty"`
	Status InputManifestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// InputManifestList contains a list of InputManifest
type InputManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InputManifest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InputManifest{}, &InputManifestList{})
}

func (pwd *ProviderWithData) GetSecretField(name SecretField) (string, error) {
	if key, ok := pwd.Secret.Data[string(name)]; ok {
		return string(key), nil
	} else {
		return "", fmt.Errorf("fields %s not found", name)
	}
}
