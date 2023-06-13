package manifest

type Manifest struct {
	Name         string       `validate:"required" yaml:"name"`
	Providers    Provider     `yaml:"providers"`
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
	Credentials string `validate:"required,json" yaml:"credentials"`
	GCPProject  string `validate:"required" yaml:"gcpProject"`
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
	AccessKey string `validate:"required,alphanum,len=20" yaml:"accessKey"`
	SecretKey string `validate:"required,len=40" yaml:"secretKey"`
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
	Name             string            `validate:"required" yaml:"name"`
	ProviderSpec     ProviderSpec      `validate:"required" yaml:"providerSpec"`
	Count            int32             `validate:"required_without=AutoscalerConfig,excluded_with=AutoscalerConfig" yaml:"count"`
	ServerType       string            `validate:"required" yaml:"serverType"`
	Image            string            `validate:"required" yaml:"image"`
	StorageDiskSize  int64             `validate:"omitempty,gte=50" yaml:"storageDiskSize"`
	AutoscalerConfig AutoscalerConfig  `validate:"required_without=Count,excluded_with=Count" yaml:"autoscaler"`
	Labels           map[string]string `validate:"omitempty" yaml:"labels"`
	Taints           []Taint           `validate:"dive" yaml:"taints"`
}

type AutoscalerConfig struct {
	Min int32 `yaml:"min"`
	Max int32 `yaml:"max"`
}

type ProviderSpec struct {
	Name   string `validate:"required" yaml:"name"`
	Region string `validate:"required" yaml:"region"`
	Zone   string `validate:"required" yaml:"zone"`
}

type StaticNodePool struct {
	Name   string            `validate:"required" yaml:"name"`
	Nodes  []Node            `validate:"dive" yaml:"nodes"`
	Labels map[string]string `validate:"omitempty" yaml:"labels"`
	Taints []Taint           `validate:"dive" yaml:"taints"`
}

type Taint struct {
	Effect string `validate:"required" yaml:"effect"`
	Value  string `validate:"omitempty" yaml:"value"`
	Key    string `validate:"required" yaml:"key"`
}

type Node struct {
	Endpoint string `validate:"required,ip_addr" yaml:"endpoint"`
	Key      string `validate:"required" yaml:"privateKey"`
}

type Cluster struct {
	Name    string `validate:"required" yaml:"name"`
	Version string `validate:"required,ver" yaml:"version"`
	Network string `validate:"required,cidrv4" yaml:"network"`
	Pools   Pool   `validate:"dive" yaml:"pools"`
}

type Pool struct {
	Control []string `validate:"min=1" yaml:"control"`
	Compute []string `yaml:"compute"`
}

type Role struct {
	Name       string `validate:"required" yaml:"name"`
	Protocol   string `validate:"required,oneof=tcp udp" yaml:"protocol"`
	Port       int32  `validate:"min=0,max=65535" yaml:"port"`
	TargetPort int32  `validate:"min=0,max=65535" yaml:"targetPort"`
	Target     string `validate:"required,oneof=k8sAllNodes k8sControlPlane k8sComputePlane" yaml:"target"`
}

type LoadBalancerCluster struct {
	Name        string   `validate:"required" yaml:"name"`
	Roles       []string `yaml:"roles"`
	DNS         DNS      `validate:"required" yaml:"dns,omitempty"`
	TargetedK8s string   `validate:"required" yaml:"targetedK8s"`
	Pools       []string `yaml:"pools"`
}

type DNS struct {
	DNSZone  string `validate:"required" yaml:"dnsZone"`
	Provider string `validate:"required" yaml:"provider"`
	Hostname string `yaml:"hostname,omitempty"`
}
