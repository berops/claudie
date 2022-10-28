package manifest

////////////////////YAML STRUCT//////////////////////////////////////////////////

type Manifest struct {
	Name         string       `validate:"required" yaml:"name"`
	Providers    Provider     `yaml:"providers"`
	NodePools    NodePool     `yaml:"nodePools"`
	Kubernetes   Kubernetes   `yaml:"kubernetes"`
	LoadBalancer LoadBalancer `yaml:"loadBalancers"`
}

type Provider struct {
	GCP     []GCP     `yaml:"gcp"`
	Hetzner []Hetzner `yaml:"hetzner"`
	AWS     []AWS     `yaml:"aws"`
	OCI     []OCI     `yaml:"oci"`
	Azure   []Azure   `yaml:"azure"`
}

type GCP struct {
	Name string `validate:"required" yaml:"name"`
	// We can only validate that the supplied string is a
	// valid formatted JSON.
	Credentials string `validate:"required,json" yaml:"credentials"`
	GCPProject  string `validate:"required" yaml:"gcp_project"`
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
	Name      string `validate:"required" yaml:"name"`
	AccessKey string `validate:"required,alphanum,len=20" yaml:"access_key"`
	SecretKey string `validate:"required,len=40" yaml:"secret_key"`
}
type OCI struct {
	Name           string `validate:"required" yaml:"name"`
	PrivateKey     string `validate:"required" yaml:"private_key"`
	KeyFingerprint string `validate:"required" yaml:"key_fingerprint"`
	TenancyOCID    string `validate:"required" yaml:"tenancy_ocid"`
	UserOCID       string `validate:"required" yaml:"user_ocid"`
	CompartmentID  string `validate:"required" yaml:"compartment_ocid"`
}

type Azure struct {
	Name           string `validate:"required" yaml:"name"`
	SubscriptionId string `validate:"required" yaml:"subscription_id"`
	TenantId       string `validate:"required" yaml:"tenant_id"`
	ClientId       string `validate:"required" yaml:"client_id"`
	ResourceGroup  string `validate:"required" yaml:"resource_group"`
	ClientSecret   string `validate:"required" yaml:"client_secret"`
}

type NodePool struct {
	Dynamic []DynamicNodePool `yaml:"dynamic"`
	Static  []StaticNodePool  `yaml:"static"`
}

type LoadBalancer struct {
	Roles    []Role                `yaml:"roles"`
	Clusters []LoadBalancerCluster `yaml:"clusters"`
}

type Kubernetes struct {
	Clusters []Cluster `yaml:"clusters"`
}

type DynamicNodePool struct {
	Name         string       `validate:"required" yaml:"name"`
	ProviderSpec ProviderSpec `validate:"required" yaml:"providerSpec"`
	Count        int64        `yaml:"count"`
	ServerType   string       `validate:"required" yaml:"server_type"`
	Image        string       `validate:"required" yaml:"image"`
	DiskSize     int64        `validate:"required" yaml:"disk_size"`
}

type ProviderSpec struct {
	Name   string `validate:"required" yaml:"name"`
	Region string `validate:"required" yaml:"region"`
	Zone   string `validate:"required" yaml:"zone"`
}

type StaticNodePool struct {
	Name  string `validate:"required" yaml:"name"`
	Nodes []Node `validate:"dive" yaml:"nodes"`
}

type Node struct {
	PublicIP      string `validate:"required,ip_addr" yaml:"publicIP"`
	PrivateSSHKey string `validate:"required" yaml:"privateSshKey"`
}

type Cluster struct {
	Name    string `validate:"required" yaml:"name"`
	Version string `validate:"required,ver" yaml:"version"`
	Network string `validate:"required,cidrv4" yaml:"network"`
	Pools   Pool   `yaml:"pools"`
}

type Pool struct {
	Control []string `yaml:"control"`
	Compute []string `yaml:"compute"`
}

type Role struct {
	Name       string `validate:"required" yaml:"name"`
	Protocol   string `validate:"required,oneof=tcp udp" yaml:"protocol"`
	Port       int32  `validate:"min=0,max=65535" yaml:"port"`
	TargetPort int32  `validate:"min=0,max=65535" yaml:"target_port"`
	Target     string `validate:"required,oneof=k8sAllNodes k8sControlPlane k8sComputePlane" yaml:"target"`
}

type LoadBalancerCluster struct {
	Name        string   `validate:"required" yaml:"name"`
	Roles       []string `yaml:"roles"`
	DNS         DNS      `validate:"required" yaml:"dns,omitempty"`
	TargetedK8s string   `validate:"required" yaml:"targeted-k8s"`
	Pools       []string `yaml:"pools"`
}

type DNS struct {
	DNSZone  string `validate:"required" yaml:"dns_zone"`
	Provider string `validate:"required" yaml:"provider"`
	Hostname string `yaml:"hostname,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
