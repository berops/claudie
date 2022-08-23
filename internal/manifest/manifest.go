package manifest

////////////////////YAML STRUCT//////////////////////////////////////////////////

type Manifest struct {
	Name         string       `yaml:"name"`
	Providers    Provider     `yaml:"providers"`
	NodePools    NodePool     `yaml:"nodePools"`
	Kubernetes   Kubernetes   `yaml:"kubernetes"`
	LoadBalancer LoadBalancer `yaml:"loadBalancers"`
}

type Provider struct {
	GCP     []GCP     `yaml:"gcp"`
	Hetzner []Hetzner `yaml:"hetzner"`
}

type GCP struct {
	Name        string `yaml:"name"`
	Credentials string `yaml:"credentials"`
	GCPProject  string `yaml:"gcp_project"`
}

type Hetzner struct {
	Name        string `yaml:"name"`
	Credentials string `yaml:"credentials"`
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
	Name         string       `yaml:"name"`
	ProviderSpec ProviderSpec `yaml:"providerSpec"`
	Count        int64        `yaml:"count"`
	ServerType   string       `yaml:"server_type"`
	Image        string       `yaml:"image"`
	DiskSize     int64        `yaml:"disk_size"`
}

type ProviderSpec struct {
	Name   string `yaml:"name"`
	Region string `yaml:"region"`
	Zone   string `yaml:"zone"`
}

type StaticNodePool struct {
	Name  string `yaml:"name"`
	Nodes []Node `yaml:"nodes"`
}

type Node struct {
	PublicIP      string `yaml:"publicIP"`
	PrivateSSHKey string `yaml:"privateSshKey"`
}

type Cluster struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Network string `yaml:"network"`
	Pools   Pool   `yaml:"pools"`
}

type Pool struct {
	Control []string `yaml:"control"`
	Compute []string `yaml:"compute"`
}

type Role struct {
	Name       string `yaml:"name"`
	Protocol   string `yaml:"protocol"`
	Port       int32  `yaml:"port"`
	TargetPort int32  `yaml:"target_port"`
	Target     string `yaml:"target"`
}

type LoadBalancerCluster struct {
	Name        string   `yaml:"name"`
	Roles       []string `yaml:"roles"`
	DNS         DNS      `yaml:"dns,omitempty"`
	TargetedK8s string   `yaml:"targeted-k8s"`
	Pools       []string `yaml:"pools"`
}

type DNS struct {
	DNSZone  string `yaml:"dns_zone"`
	Provider string `yaml:"provider"`
	Hostname string `yaml:"hostname,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
