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
	"github.com/berops/claudie/internal/api/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderType type of a provider.
// A list of available providers can be found at https://docs.claudie.io/latest/input-manifest/providers/aws/
type ProviderType string

const (
	AWS        ProviderType = "aws"
	AZURE      ProviderType = "azure"
	CLOUDFLARE ProviderType = "cloudflare"
	GCP        ProviderType = "gcp"
	HETZNER    ProviderType = "hetzner"
	OCI        ProviderType = "oci"
	OPENSTACK  ProviderType = "openstack"
	EXOSCALE   ProviderType = "exoscale"
)

type SecretField string

const (
	AWS_ACCESS_KEY                   SecretField = "accesskey"
	AWS_SECRET_KEY                   SecretField = "secretkey"
	AZURE_CLIENT_SECRET              SecretField = "clientsecret"
	AZURE_SUBSCRIPTION_ID            SecretField = "subscriptionid"
	AZURE_TENANT_ID                  SecretField = "tenantid"
	AZURE_CLIENT_ID                  SecretField = "clientid"
	CF_API_TOKEN                     SecretField = "apitoken"
	CF_ACCOUNT_ID                    SecretField = "accountid"
	GCP_CREDENTIALS                  SecretField = "credentials"
	GCP_GCP_PROJECT                  SecretField = "gcpproject"
	HETZNER_CREDENTIALS              SecretField = "credentials"
	HETZNER_DNS_API_TOKEN            SecretField = "apitoken"
	OCI_PRIVATE_KEY                  SecretField = "privatekey"
	OCI_KEY_FINGERPRINT              SecretField = "keyfingerprint"
	OCI_TENANCT_OCID                 SecretField = "tenancyocid"
	OCI_USER_OCID                    SecretField = "userocid"
	OCI_COMPARTMENT_OCID             SecretField = "compartmentocid"
	PRIVATE_KEY                      SecretField = "privatekey"
	OS_AUTH_URL                      SecretField = "authurl"
	OS_DOMAIN_ID                     SecretField = "domainid"
	OS_PROJECT_ID                    SecretField = "projectid"
	OS_APPLICATION_CREDENTIAL_ID     SecretField = "applicationcredentialid"
	OS_APPLICATION_CREDENTIAL_SECRET SecretField = "applicationcredentialsecret"
	EXOSCALE_API_KEY                 SecretField = "apikey"
	EXOSCALE_API_SECRET              SecretField = "apisecret"
)

// ProviderWithData helper type that assist in conversion
// from SecretReference to Secret
type ProviderWithData struct {
	ProviderName string
	ProviderType ProviderType
	Secret       corev1.Secret
	Templates    *manifest.TemplateRepository
}

type StaticNodeWithData struct {
	Endpoint string
	Username string
	Secret   corev1.Secret
}

// Providers list of defined cloud provider configuration
// that will be used while infrastructure provisioning.
type Provider struct {
	// Name is the name of the provider specification. It has to be unique across all providers.
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:MinLength=1
	ProviderName string `json:"name"`
	// +kubebuilder:validation:Enum=gcp;hetzner;aws;oci;azure;cloudflare;openstack;exoscale;
	ProviderType ProviderType           `json:"providerType"`
	SecretRef    corev1.SecretReference `json:"secretRef"`
	// External templates for building the cluster infrastructure.
	// +optional
	Templates *manifest.TemplateRepository `json:"templates"`
}

// NodePool is a map of dynamic nodepools and static nodepools which will be used to
// form kubernetes or loadbalancer clusters.
type NodePool struct {
	// Dynamic nodepools define nodepools dynamically created by Claudie.
	// +optional
	Dynamic []manifest.DynamicNodePool `json:"dynamic"`
	// Static nodepools define nodepools of already existing nodes.
	// +optional
	Static []StaticNodePool `json:"static"`
}

// StaticNodePool defines nodepool of already existing nodes, managed outside of Claudie.
type StaticNodePool struct {
	// Name of the nodepool.
	Name string `json:"name"`
	// List of static nodes for a particular static nodepool.
	Nodes []StaticNode `json:"nodes"`
	// +optional
	Labels map[string]string `json:"labels"`
	// +optional
	Annotations map[string]string `json:"annotations"`
	// +optional
	Taints []corev1.Taint `json:"taints"`
}

// StaticNode defines a single static node for a particular static nodepool.
type StaticNode struct {
	// Endpoint under which Claudie will access this node.
	Endpoint string `json:"endpoint"`
	// Secret reference to the private key of the node.
	SecretRef corev1.SecretReference `json:"secretRef"`
	// Username with root access. Used in SSH connection also.
	Username string `json:"username,omitempty"`
}

// Role defines a concrete loadbalancer configuration. A Single loadbalancer can have multiple roles.
type Role struct {
	// Name of the role. Used as a reference in clusters.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Protocol of the rule. Allowed values are: tcp, udp.
	// +kubebuilder:validation:Enum=tcp;udp;
	Protocol string `json:"protocol"`

	// Port of the incoming traffic on the loadbalancer.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Port where loadbalancer forwards the traffic.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	TargetPort int32 `json:"targetPort"`

	// Defines nodepools of the targeted K8s cluster, from which nodes will be used for loadbalancing.
	// +kubebuilder:validation:MinItems=1
	TargetPools []string `json:"targetPools"`

	// Additional settings for a role.
	// +optional
	Settings *manifest.RoleSettings `yaml:"settings,omitempty" json:"settings,omitempty"`
}

type LoadBalancer struct {
	// List of roles loadbalancers use to forward the traffic. Single role can be used in multiple loadbalancer clusters.
	// +optional
	Roles []Role `yaml:"roles" json:"roles"`
	// A list of load balancers clusters.
	// +optional
	Clusters []manifest.LoadBalancerCluster `yaml:"clusters" json:"clusters"`
}

// Specification of the desired behaviour of the InputManifest
type InputManifestSpec struct {
	// Providers list of defined cloud provider configuration
	// that will be used while infrastructure provisioning.
	// +optional
	Providers []Provider `json:"providers,omitempty"`
	// +optional
	NodePools NodePool `json:"nodePools,omitzero"`
	// +optional
	Kubernetes manifest.Kubernetes `json:"kubernetes,omitzero"`
	// +optional
	LoadBalancer LoadBalancer `json:"loadBalancers,omitzero"`
}

// Most recently observed status of the InputManifest
type InputManifestStatus struct {
	State    string                    `json:"state,omitempty"`
	Clusters map[string]ClustersStatus `json:"clusters,omitempty"`
}

type FinishedWorkflow struct {
	Status          string `json:"status,omitempty"`
	Stage           string `json:"stage,omitempty"`
	TaskDescription string `json:"taskDescription,omitempty"`
	Timestamp       string `json:"timestamp"`
}

type ClustersStatus struct {
	State    string             `json:"state,omitempty"`
	Phase    string             `json:"phase,omitempty"`
	Message  string             `json:"message,omitempty"`
	Previous []FinishedWorkflow `json:"previous"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Status of the input manifest"
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels=app.kubernetes.io/part-of=claudie
// InputManifest is a definition of the user's infrastructure.
// It contains cloud provider specification,
// nodepool specification, Kubernetes and loadbalancer clusters.
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
