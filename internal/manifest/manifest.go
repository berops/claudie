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

// Nodepools field is used for defining the nodepool specification.
// You can think of them as a blueprints, not actual nodepools that will be created
type NodePool struct {
	Dynamic []DynamicNodePool `yaml:"dynamic" json:"dynamic"`
	// +optional
	Static  []StaticNodePool  `yaml:"static" json:"static"`
}

type LoadBalancer struct {
	// +optional
	Roles    []Role                `yaml:"roles" json:"roles"`
	// +optional
	Clusters []LoadBalancerCluster `yaml:"clusters" json:"clusters"`
}

type Kubernetes struct {
	// +optional
	Clusters []Cluster `yaml:"clusters" json:"clusters"`
}

type DynamicNodePool struct {
	Name             string           `validate:"required" yaml:"name" json:"name"`
	ProviderSpec     ProviderSpec     `validate:"required" yaml:"providerSpec" json:"providerSpec"`
	Count            int32            `validate:"required_without=AutoscalerConfig,excluded_with=AutoscalerConfig" yaml:"count" json:"count"`
	ServerType       string           `validate:"required" yaml:"serverType" json:"serverType"`
	Image            string           `validate:"required" yaml:"image" json:"image"`
	// +optional
	StorageDiskSize  int64            `validate:"omitempty,gte=50" yaml:"storageDiskSize" json:"storageDiskSize"`
	// +optional
	AutoscalerConfig AutoscalerConfig `validate:"required_without=Count,excluded_with=Count" yaml:"autoscaler" json:"autoscaler"`
}

type AutoscalerConfig struct {
	// +optional
	Min int32 `yaml:"min" json:"min"`
	// +optional
	Max int32 `yaml:"max" json:"max"`
}

type ProviderSpec struct {
	Name   string `validate:"required" yaml:"name" json:"name"`
	Region string `validate:"required" yaml:"region" json:"region"`
	Zone   string `validate:"required" yaml:"zone" json:"zone"`
}

type StaticNodePool struct {
	Name  string `validate:"required" yaml:"name" json:"name"`
	Nodes []Node `validate:"dive" yaml:"nodes" json:"nodes"`
}

type Node struct {
	PublicIP      string `validate:"required,ip_addr" yaml:"publicIP" json:"publicIP"`
	PrivateSSHKey string `validate:"required" yaml:"privateSshKey" json:"privateSshKey"`
}

type Cluster struct {
	Name    string `validate:"required" yaml:"name" json:"name"`
	Version string `validate:"required,ver" yaml:"version" json:"version"`
	Network string `validate:"required,cidrv4" yaml:"network" json:"network"`
	Pools   Pool   `validate:"dive" yaml:"pools" json:"pools"`
}

type Pool struct {
	Control []string `validate:"min=1" yaml:"control" json:"control"`
	Compute []string `yaml:"compute" json:"compute"`
}

type Role struct {
	Name       string `validate:"required" yaml:"name" json:"name"`
	Protocol   string `validate:"required,oneof=tcp udp" yaml:"protocol" json:"protocol"`
	Port       int32  `validate:"min=0,max=65535" yaml:"port" json:"port"`
	TargetPort int32  `validate:"min=0,max=65535" yaml:"targetPort" json:"targetPort"`
	Target     string `validate:"required,oneof=k8sAllNodes k8sControlPlane k8sComputePlane" yaml:"target" json:"target"`
}

type LoadBalancerCluster struct {
	Name        string   `validate:"required" yaml:"name" json:"name"`
	Roles       []string `yaml:"roles" json:"roles"`
	DNS         DNS      `validate:"required" yaml:"dns,omitempty" json:"dns,omitempty"`
	TargetedK8s string   `validate:"required" yaml:"targetedK8s" json:"targetedK8s"`
	Pools       []string `yaml:"pools" json:"pools"`
}

type DNS struct {
	DNSZone  string `validate:"required" yaml:"dnsZone" json:"dnsZone"`
	Provider string `validate:"required" yaml:"provider" json:"provider"`
	Hostname string `yaml:"hostname,omitempty" json:"hostname,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
