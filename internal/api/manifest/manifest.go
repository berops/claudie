package manifest

import (
	k8sV1 "k8s.io/api/core/v1"
)

type TemplateRepository struct {
	Repository string `validate:"required,url" yaml:"repository" json:"repository"`
	Path       string `validate:"required,filepath" yaml:"path" json:"path"`
	// +optional
	Tag *string `validate:"omitempty,semver2" yaml:"tag" json:"tag"`
}

type Manifest struct {
	Name         string       `validate:"required" yaml:"name"`
	Providers    Provider     `yaml:"providers" json:"providers"`
	NodePools    NodePool     `yaml:"nodePools"`
	Kubernetes   Kubernetes   `yaml:"kubernetes"`
	LoadBalancer LoadBalancer `yaml:"loadBalancers"`
}

type Provider struct {
	GCP          []GCP          `yaml:"gcp"`
	Hetzner      []Hetzner      `yaml:"hetzner"`
	AWS          []AWS          `yaml:"aws"`
	OCI          []OCI          `yaml:"oci"`
	Azure        []Azure        `yaml:"azure"`
	Cloudflare   []Cloudflare   `yaml:"cloudflare"`
	HetznerDNS   []HetznerDNS   `yaml:"hetznerdns"`
	GenesisCloud []GenesisCloud `yaml:"genesiscloud"`
}

type HetznerDNS struct {
	Name      string              `validate:"required,max=15" yaml:"name"`
	ApiToken  string              `validate:"required" yaml:"apiToken"`
	Templates *TemplateRepository `validate:"omitempty" yaml:"templates" json:"templates"`
}

type Cloudflare struct {
	Name      string              `validate:"required,max=15" yaml:"name"`
	ApiToken  string              `validate:"required" yaml:"apiToken"`
	AccountID string              `validate:"required" yaml:"accountId"`
	Templates *TemplateRepository `validate:"omitempty" yaml:"templates" json:"templates"`
}

type GCP struct {
	Name string `validate:"required,max=15" yaml:"name"`
	// We can only validate that the supplied string is a
	// valid formatted JSON.
	Credentials string              `validate:"required,json" yaml:"credentials" json:"credentials"`
	GCPProject  string              `validate:"required" yaml:"gcpProject" json:"gcpProject"`
	Templates   *TemplateRepository `validate:"omitempty" yaml:"templates" json:"templates"`
}

type Hetzner struct {
	Name string `validate:"required,max=15" yaml:"name"`

	// We can only validate the length of the token
	// as Hetzner doesn't specify the structure of the token,
	// only that it's a hash. We can also validate that the characters
	// are alphanumeric (i.e. excluding characters like !#@$%^&*...)
	// https://docs.hetzner.com/cloud/technical-details/faq#how-are-api-tokens-stored
	Credentials string              `validate:"required,alphanum,len=64" yaml:"credentials"`
	Templates   *TemplateRepository `validate:"omitempty" yaml:"templates" json:"templates"`
}

type GenesisCloud struct {
	Name      string              `validate:"required,max=15" yaml:"name"`
	ApiToken  string              `validate:"required,alphanum" yaml:"apiToken"`
	Templates *TemplateRepository `validate:"omitempty" yaml:"templates" json:"templates"`
}

type AWS struct {
	Name      string              `validate:"required,max=15" yaml:"name" json:"name"`
	AccessKey string              `validate:"required,alphanum,len=20" yaml:"accessKey" json:"accessKey"`
	SecretKey string              `validate:"required,len=40" yaml:"secretKey" json:"secretKey"`
	Templates *TemplateRepository `validate:"omitempty" yaml:"templates" json:"templates"`
}
type OCI struct {
	Name           string              `validate:"required,max=15" yaml:"name"`
	PrivateKey     string              `validate:"required" yaml:"privateKey"`
	KeyFingerprint string              `validate:"required" yaml:"keyFingerprint"`
	TenancyOCID    string              `validate:"required" yaml:"tenancyOcid"`
	UserOCID       string              `validate:"required" yaml:"userOcid"`
	CompartmentID  string              `validate:"required" yaml:"compartmentOcid"`
	Templates      *TemplateRepository `validate:"omitempty" yaml:"templates" json:"templates"`
}

type Azure struct {
	Name           string              `validate:"required,max=15" yaml:"name"`
	SubscriptionId string              `validate:"required" yaml:"subscriptionId"`
	TenantId       string              `validate:"required" yaml:"tenantId"`
	ClientId       string              `validate:"required" yaml:"clientId"`
	ClientSecret   string              `validate:"required" yaml:"clientSecret"`
	Templates      *TemplateRepository `validate:"omitempty" yaml:"templates" json:"templates"`
}

// NodePools describes nodepools used for either kubernetes clusters
// or loadbalancer cluster defined in this manifest.
type NodePool struct {
	// List of dynamically to-be-created nodepools of not yet existing machines, used for Kubernetes or loadbalancer clusters.
	// +optional
	Dynamic []DynamicNodePool `yaml:"dynamic" json:"dynamic"`
	// List of static nodepools of already existing machines, not created by Claudie, used for Kubernetes or loadbalancer clusters.
	// +optional
	Static []StaticNodePool `yaml:"static" json:"static"`
}

// LoadBalancers list of loadbalancer clusters the Kubernetes clusters may use.
type LoadBalancer struct {
	// List of roles loadbalancers use to forward the traffic. Single role can be used in multiple loadbalancer clusters.
	// +optional
	Roles []Role `yaml:"roles" json:"roles"`
	// A list of load balancers clusters.
	// +optional
	Clusters []LoadBalancerCluster `yaml:"clusters" json:"clusters"`
}

// Kubernetes list of Kubernetes cluster this manifest will manage.
type Kubernetes struct {
	// List of Kubernetes clusters Claudie will create.
	// +optional
	Clusters []Cluster `yaml:"clusters" json:"clusters"`
}

// MachineSpec specifies further the configuration of the requested server type in DynamicNodePool.
type MachineSpec struct {
	// CpuCount specifies the number of CPU cores the provided instance type will have.
	// +optional
	CpuCount int `validate:"required_with=Memory,gte=0" yaml:"cpuCount" json:"cpuCount"`
	// Memory specifies the memory the provided instance type will have.
	// +optional
	Memory int `validate:"required_with=CpuCount,gte=0" yaml:"memory" json:"memory"`
	// Nvidia specifies the number of NVIDIA GPUs the provided instance type will have.
	// +optional
	NvidiaGpu int `validate:"gte=0" yaml:"nvidiaGpu" json:"nvidiaGpu"`
}

// DynamicNodePool List of dynamically to-be-created nodepools of not yet existing machines, used for Kubernetes or loadbalancer clusters.
// These are only blueprints, and will only be created per reference in kubernetes or loadBalancer clusters.
//
// E.g. if the nodepool isn't used, it won't even be created. Or if the same nodepool is used in two different clusters,
// it will be created twice. In OOP analogy, a dynamic nodepool would be a class
// that would get instantiated N >= 0 times depending on which clusters reference it.
type DynamicNodePool struct {
	// Name of the nodepool. Each nodepool will have a random hash appended to the name, so the whole name will be of format <name>-<hash>.
	Name string `validate:"required,max=14" yaml:"name" json:"name"`
	// Collection of provider data to be used while creating the nodepool.
	ProviderSpec ProviderSpec `validate:"required" yaml:"providerSpec" json:"providerSpec"`
	// Number of the nodes in the nodepool. Mutually exclusive with autoscaler.
	// +optional
	Count int32 `validate:"required_without=AutoscalerConfig,excluded_with=AutoscalerConfig" yaml:"count" json:"count,omitempty"`
	// 	Type of the machines in the nodepool. Currently, only AMD64 machines are supported.
	ServerType string `validate:"required" yaml:"serverType" json:"serverType"`
	// OS image of the machine. Currently, only Ubuntu 22.04 AMD64 images are supported.
	Image string `validate:"required" yaml:"image" json:"image"`
	// Size of the storage disk on the nodes in the nodepool in GB. The OS disk is created automatically
	// with predefined size of 100GB for kubernetes nodes and 50GB for Loadbalancer nodes.
	// The value must be either -1 (no disk is created), or >= 50. If no value is specified, 50 is used.
	// +optional
	StorageDiskSize *int32 `validate:"omitempty" yaml:"storageDiskSize" json:"storageDiskSize,omitempty"`
	// Autoscaler configuration for this nodepool. Mutually exclusive with count.
	// +optional
	AutoscalerConfig AutoscalerConfig `validate:"required_without=Count,excluded_with=Count" yaml:"autoscaler" json:"autoscaler,omitempty"`
	// User defined labels for this nodepool.
	// +optional
	Labels map[string]string `validate:"omitempty" yaml:"labels" json:"labels"`
	// User defined annotations for this nodepool.
	// +optional
	Annotations map[string]string `validate:"omitempty" yaml:"annotations" json:"annotations"`
	// User defined taints for this nodepool.
	// +optional
	Taints []k8sV1.Taint `validate:"omitempty" yaml:"taints" json:"taints"`
	// MachineSpec further describe the properties of the selected server type.
	MachineSpec *MachineSpec `validate:"omitempty" yaml:"machineSpec,omitempty" json:"machineSpec,omitempty"`
}

// Autoscaler configuration on per nodepool basis. Defines the number of nodes, autoscaler will scale up or down specific nodepool.
type AutoscalerConfig struct {
	// Minimum number of nodes in nodepool.
	Min int32 `yaml:"min" json:"min,omitempty"`
	// Maximum number of nodes in nodepool.
	Max int32 `validate:"max=255" yaml:"max" json:"max,omitempty"`
}

// Provider spec is further specification build on top of the data from any of the provider instance.
type ProviderSpec struct {
	// Name of the provider instance specified in providers
	Name string `validate:"required" yaml:"name" json:"name"`
	// Region of the nodepool.
	Region string `validate:"required" yaml:"region" json:"region"`
	// Zone of the nodepool.
	// +optional
	Zone string `yaml:"zone" json:"zone"`
}

// StaticNodePool List of static nodepools of already existing machines, not created by Claudie, used for Kubernetes or loadbalancer clusters.
type StaticNodePool struct {
	// Name of the static nodepool.
	Name string `validate:"required,max=14" yaml:"name" json:"name"`
	// List of static nodes assigned to a particular nodepool.
	Nodes []Node `validate:"dive" yaml:"nodes" json:"nodes"`
	// User defined labels for this nodepool.
	// +optional
	Labels map[string]string `validate:"omitempty" yaml:"labels" json:"labels"`
	// User defined annotations for this nodepool.
	// +optional
	Annotations map[string]string `validate:"omitempty" yaml:"annotations" json:"annotations"`
	// User defined taints for this nodepool.
	// +optional
	Taints []k8sV1.Taint `validate:"omitempty" yaml:"taints" json:"taints"`
}

// Node represents a static node assigned to a particular static nodepool.
type Node struct {
	// Endpoint under which Claudie will connect to the node.
	Endpoint string `validate:"required,ip_addr" yaml:"endpoint" json:"endpoint"`
	// Private key used to ssh into the node.
	Key string `validate:"required" yaml:"privateKey" json:"privateKey"`
	// Username with root access. Used in SSH connection also.
	Username string `validate:"required" yaml:"username" json:"username"`
}

// Collection of data used to define a Kubernetes cluster.
type Cluster struct {
	// Name of the Kubernetes cluster. Each cluster will have a random hash appended to the name, so the whole name will be of format <name>-<hash>.
	Name string `validate:"required,max=28" yaml:"name" json:"name"`
	// Version should be defined in format vX.Y. In terms of supported versions of Kubernetes,
	// Claudie follows kubeone releases and their supported versions.
	// The current kubeone version used in Claudie is 1.8.1.
	// To see the list of supported versions, please refer to kubeone documentation.
	// https://docs.kubermatic.com/kubeone/v1.8/architecture/compatibility/supported-versions/
	Version string `validate:"required,ver" yaml:"version" json:"version"`
	// Network range for the VPN of the cluster. The value should be defined in format A.B.C.D/mask.
	Network string `validate:"required,cidrv4" yaml:"network" json:"network"`
	// List of nodepool names this cluster will use.
	Pools Pool `yaml:"pools" json:"pools"`
	// General information about a proxy used to build a K8s cluster.
	InstallationProxy *InstallationProxy `yaml:"installationProxy,omitempty" json:"installationProxy,omitempty"`
}

// List of nodepool names this cluster will use. Remember that nodepools defined in nodepools
// are only "blueprints". The actual nodepool will be created once referenced here.
type Pool struct {
	// List of nodepool names, that will represent control plane nodes.
	Control []string `validate:"min=1" yaml:"control" json:"control"`
	// List of nodepool names, that will represent compute nodes.
	Compute []string `yaml:"compute" json:"compute"`
}

// General information about a proxy used to build a K8s cluster.
type InstallationProxy struct {
	// Mode defines if the proxy mode (on/off/default). If undefined, the default mode is used.
	Mode string `validate:"required,proxyMode" default:"default" yaml:"mode" json:"mode"`
	// Endpoint defines the proxy endpoint. If undefined, the default value is http://proxy.claudie.io:8880.
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	// NoProxy is a comma-separated list of values that will be added to the default NoProxy list used by Claudie.
	//
	// The default no proxy list is: 127.0.0.1/8,localhost,cluster.local,10.244.0.0/16,10.96.0.0/12"
	// Any values specified will be appended to the end of the default NoProxy list.
	// This field only has an effect if the Proxy is turned on.
	NoProxy string `yaml:"noProxy,omitempty" json:"noProxy,omitempty"`
}

// Additional settings for a role.
type RoleSettings struct {
	ProxyProtocol  bool `yaml:"proxyProtocol" json:"proxyProtocol"`
	StickySessions bool `yaml:"stickySessions" json:"stickySessions"`
}

// Role defines a concrete loadbalancer configuration. Single loadbalancer can have multiple roles.
type Role struct {
	// Name of the role. Used as a reference in clusters.
	Name string `validate:"required" yaml:"name" json:"name"`
	// Protocol of the rule. Allowed values are: tcp, udp.
	Protocol string `validate:"required,oneof=tcp udp" yaml:"protocol" json:"protocol"`
	// Port of the incoming traffic on the loadbalancer.
	Port int32 `validate:"min=0,max=65535" yaml:"port" json:"port"`
	// Port where loadbalancer forwards the traffic.
	TargetPort int32 `validate:"min=0,max=65535" yaml:"targetPort" json:"targetPort"`
	// Defines nodepools of the targeted K8s cluster, from which nodes will be used for loadbalancing.
	TargetPools []string `validate:"required,min=1" yaml:"targetPools" json:"targetPools"`
	// Additional settings for a role.
	Settings *RoleSettings `yaml:"settings,omitempty" json:"settings,omitempty"`
}

// Collection of data used to define a loadbalancer cluster. Defines loadbalancer clusters.
type LoadBalancerCluster struct {
	// Name of the loadbalancer.
	Name string `validate:"required,max=28" yaml:"name" json:"name"`
	// List of roles the loadbalancer uses.
	Roles []string `yaml:"roles" json:"roles"`
	// Specification of the loadbalancer's DNS record.
	DNS DNS `validate:"required" yaml:"dns,omitempty" json:"dns,omitempty"`
	// Name of the Kubernetes cluster targeted by this loadbalancer.
	TargetedK8s string `validate:"required" yaml:"targetedK8s" json:"targetedK8s"`
	// List of nodepool names this loadbalancer will use. Remember, that nodepools defined
	// in nodepools are only "blueprints". The actual nodepool will be created once referenced here.
	Pools []string `yaml:"pools" json:"pools"`
}

// Collection of data Claudie uses to create a DNS record for the loadbalancer.
type DNS struct {
	// DNS zone inside of which the records will be created. GCP/AWS/OCI/Azure/Cloudflare/Hetzner DNS zone is accepted
	DNSZone string `validate:"required" yaml:"dnsZone" json:"dnsZone"`
	// Name of provider to be used for creating an A record entry in defined DNS zone.
	Provider string `validate:"required" yaml:"provider" json:"provider"`
	// Custom hostname for your A record. If left empty, the hostname will be a random hash.
	Hostname string `yaml:"hostname,omitempty" json:"hostname,omitempty"`
	// Alternative names that will be created in addition to the hostname. Giving the ability
	// to have a loadbalancer for multiple hostnames.
	//
	// - api.example.com
	//
	// - apiv2.example.com
	// +optional
	AlternativeNames []string `validate:"dive,required" yaml:"alternativeNames,omitempty" json:"alternativeNames,omitempty"`
}
