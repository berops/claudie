package manifest

////////////////////YAML STRUCT//////////////////////////////////////////////////

type Manifest struct {
	Name         string       `validate:"required" yaml:"name"`
	Providers    Provider     `yaml:"providers" json:"providers"`
	NodePools    NodePool     `yaml:"nodePools"`
	Kubernetes   Kubernetes   `yaml:"kubernetes"`
	LoadBalancer LoadBalancer `yaml:"loadBalancers"`
}

type Provider struct {
	GCP        []GCP        `yaml:"gcp"`
	Hetzner    []Hetzner    `yaml:"hetzner"`
	AWS        []AWS        `yaml:"aws"`
	OCI        []OCI        `yaml:"oci"`
	Azure      []Azure      `yaml:"azure"`
	Cloudflare []Cloudflare `yaml:"cloudflare"`
	HetznerDNS []HetznerDNS `yaml:"hetznerdns"`
}

type HetznerDNS struct {
	Name     string `validate:"required" yaml:"name"`
	ApiToken string `validate:"required" yaml:"apiToken"`
}

type Cloudflare struct {
	Name     string `validate:"required" yaml:"name"`
	ApiToken string `validate:"required" yaml:"apiToken"`
}

type GCP struct {
	Name string `validate:"required" yaml:"name"`
	// We can only validate that the supplied string is a
	// valid formatted JSON.
	Credentials string `validate:"required,json" yaml:"credentials" json:"credentials"`
	GCPProject  string `validate:"required" yaml:"gcpProject" json:"gcpProject"`
}

type Hetzner struct {
	Name string `validate:"required" yaml:"name"`

	// We can only validate the length of the token
	// as Hetzner doesn't specify the structure of the token,
	// only that it's a hash. We can also validate that the characters
	// are alphanumeric (i.e. excluding characters like !#@$%^&*...)
	// https://docs.hetzner.com/cloud/technical-details/faq#how-are-api-tokens-stored
	Credentials string `validate:"required,alphanum,len=64" yaml:"credentials"`
}

type AWS struct {
	Name      string `validate:"required" yaml:"name" json:"name"`
	AccessKey string `validate:"required,alphanum,len=20" yaml:"accessKey" json:"accessKey"`
	SecretKey string `validate:"required,len=40" yaml:"secretKey" json:"secretKey"`
}
type OCI struct {
	Name           string `validate:"required" yaml:"name"`
	PrivateKey     string `validate:"required" yaml:"privateKey"`
	KeyFingerprint string `validate:"required" yaml:"keyFingerprint"`
	TenancyOCID    string `validate:"required" yaml:"tenancyOcid"`
	UserOCID       string `validate:"required" yaml:"userOcid"`
	CompartmentID  string `validate:"required" yaml:"compartmentOcid"`
}

type Azure struct {
	Name           string `validate:"required" yaml:"name"`
	SubscriptionId string `validate:"required" yaml:"subscriptionId"`
	TenantId       string `validate:"required" yaml:"tenantId"`
	ClientId       string `validate:"required" yaml:"clientId"`
	ClientSecret   string `validate:"required" yaml:"clientSecret"`
}

// NodePools describes nodepools used for either kubernetes clusters
// or loadbalancer cluster defined in this manifest.
type NodePool struct {
	// List of dynamically to-be-created nodepools of not yet existing machines, used for Kubernetes or loadbalancer clusters.
	// +optional
	Dynamic []DynamicNodePool `yaml:"dynamic" json:"dynamic"`
	// [WORK IN PROGRESS] List of static nodepools of already existing machines, not created by of Claudie, used for Kubernetes or loadbalancer clusters.
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

// DynamicNodePool List of dynamically to-be-created nodepools of not yet existing machines, used for Kubernetes or loadbalancer clusters.
// These are only blueprints, and will only be created per reference in kubernetes or loadBalancer clusters.
//
// E.g. if the nodepool isn't used, it won't even be created. Or if the same nodepool is used in two different clusters,
// it will be created twice. In OOP analogy, a dynamic nodepool would be a class
// that would get instantiated N >= 0 times depending on which clusters reference it.
type DynamicNodePool struct {
	// Name of the nodepool. Each nodepool will have a random hash appended to the name, so the whole name will be of format <name>-<hash>.
	Name string `validate:"required" yaml:"name" json:"name"`
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
	// +optional
	StorageDiskSize int64 `validate:"omitempty,gte=50" yaml:"storageDiskSize" json:"storageDiskSize,omitempty"`
	// Autoscaler configuration for this nodepool. Mutually exclusive with count.
	// +optional
	AutoscalerConfig AutoscalerConfig `validate:"required_without=Count,excluded_with=Count" yaml:"autoscaler" json:"autoscaler,omitempty"`
}

// Autoscaler configuration on per nodepool basis. Defines the number of nodes, autoscaler will scale up or down specific nodepool.
type AutoscalerConfig struct {
	// Minimum number of nodes in nodepool.
	Min int32 `yaml:"min" json:"min,omitempty"`
	// Maximum number of nodes in nodepool.
	Max int32 `yaml:"max" json:"max,omitempty"`
}

// Provider spec is further specification build on top of the data from any of the provider instance.
type ProviderSpec struct {
	// Name of the provider instance specified in providers
	Name string `validate:"required" yaml:"name" json:"name"`
	// Region of the nodepool.
	Region string `validate:"required" yaml:"region" json:"region"`
	// Zone of the nodepool.
	Zone string `validate:"required" yaml:"zone" json:"zone"`
}

// WIP
type StaticNodePool struct {
	Name  string `validate:"required" yaml:"name" json:"name"`
	Nodes []Node `validate:"dive" yaml:"nodes" json:"nodes"`
}

// WIP
type Node struct {
	Endpoint string `validate:"required,ip_addr" yaml:"endpoint" json:"endpoint"`
	Key      string `validate:"required" yaml:"privateKey" json:"privatekey"`
}

// Collection of data used to define a Kubernetes cluster.
type Cluster struct {
	// Name of the Kubernetes cluster. Each cluster will have a random hash appended to the name, so the whole name will be of format <name>-<hash>.
	Name string `validate:"required" yaml:"name" json:"name"`
	// Version should be defined in format vX.Y. In terms of supported versions of Kubernetes,
	// Claudie follows kubeone releases and their supported versions.
	// The current kubeone version used in Claudie is 1.5.
	// To see the list of supported versions, please refer to kubeone documentation.
	// https://docs.kubermatic.com/kubeone/v1.5/architecture/compatibility/supported-versions/#supported-kubernetes-versions
	Version string `validate:"required,ver" yaml:"version" json:"version"`
	// Network range for the VPN of the cluster. The value should be defined in format A.B.C.D/mask.
	Network string `validate:"required,cidrv4" yaml:"network" json:"network"`
	// List of nodepool names this cluster will use.
	Pools Pool `validate:"dive" yaml:"pools" json:"pools"`
}

// List of nodepool names this cluster will use. Remember that nodepools defined in nodepools
// are only "blueprints". The actual nodepool will be created once referenced here.
type Pool struct {
	// List of nodepool names, that will represent control plane nodes.
	Control []string `validate:"min=1" yaml:"control" json:"control"`
	// List of nodepool names, that will represent compute nodes.
	Compute []string `yaml:"compute" json:"compute"`
}

// Role defines a concrete loadbalancer configuration. Single loadbalancer can have multiple roles.
type Role struct {
	// Name of the role. Used as a reference in clusters.
	Name string `validate:"required" yaml:"name" json:"name"`
	// Protocol of the rule. Allowed values are: tcp, udp.
	// +kubebuilder:validation:Enum=tcp;udp;
	Protocol string `validate:"required,oneof=tcp udp" yaml:"protocol" json:"protocol"`
	// Port of the incoming traffic on the loadbalancer.
	Port int32 `validate:"min=0,max=65535" yaml:"port" json:"port"`
	// Port where loadbalancer forwards the traffic.
	TargetPort int32 `validate:"min=0,max=65535" yaml:"targetPort" json:"targetPort"`
	// Defines a target group of nodes. Allowed values are:
	// k8sAllNodes - All nodes in the cluster
	// k8sControlNodes - Only control/master nodes in cluster
	// k8sComputeNodes - Only compute/worker nodes in cluster
	// +kubebuilder:validation:Enum=k8sAllNodes;k8sControlPlane;k8sComputePlane;
	Target string `validate:"required,oneof=k8sAllNodes k8sControlPlane k8sComputePlane" yaml:"target" json:"target"`
}

// Collection of data used to define a loadbalancer cluster. Defines loadbalancer clusters.
type LoadBalancerCluster struct {
	// Name of the loadbalancer.
	Name string `validate:"required" yaml:"name" json:"name"`
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
}

////////////////////////////////////////////////////////////////////////////////
