package templates

import "github.com/berops/claudie/proto/pb/spec"

// All the following types grouped are "Helper" types to group related data into a single context.
type (
	K8sData struct{ HasAPIServer bool }
	LBData  struct{ Roles []*spec.Role }
	IPData  struct{ V4 string }

	// RecordData is a simple wrapper containing related data
	// for DNS records to be created.
	RecordData struct {
		IP []IPData
	}

	// ClusterData wraps the assigned identifiers of a cluster specified in the InputManifest.
	ClusterData struct {
		// ClusterName is the name of the cluster as specified in the InputManifest.
		ClusterName string
		// ClusterHash is the randomly generated hash that is appended to the ClusterName.
		ClusterHash string
		// ClusterType specifies whether a Loadbalancer "LB" or Kubernetes "K8s" cluster is in the context.
		ClusterType string
	}

	// NodePoolInfo wraps data specified in the input manifest for a given nodepool and nodes of that nodepool.
	// The nodes are partially initialized by Claudie at first, with only the Name of each respective node
	// being valid. The other details such as PublicIP, PrivateIP will be initialized at a later stage
	// of the pipeline.
	NodePoolInfo struct {
		// Name is the assigned name of the nodepool along with the generated Hash.
		Name string
		// Details are the details of the Nodepool as specified in the InputManifest.
		// The CIDR field of each nodepool is initialized before the Templates are Generated
		// and therefore can be used within the templates.
		Details *spec.DynamicNodePool
		// Nodes are nodes of the dynamic nodepool specified by Details. Each node is only partially
		// initialized with only the Name of the node available during the template generation. The PublicIP of the
		// node is acquired after the infrastructure is spawned by the generated Templates. The private IP will
		// be assigned at a later stage in the pipeline when the VPN between the nodes is created.
		Nodes []*spec.Node
		// IsControl Specifies whether the nodepools is used as a control or compute nodepool within the cluster.
		// In the context of LB cluster, nodepools can only be compute or "worker" nodepools.
		IsControl bool
	}
)

// All the following types grouped are passed in as "Inputs" when generating terraform templates.
type (
	// Provider wraps all data related to generating terraform files for a Cloud provider scoped
	// only for the provider block. This structure is used when generating templates files inside the
	// provider directory of a Template repository.
	Provider struct {
		// ClusterData wraps the identifiers of the current build cluster
		// which can be either K8s or Loadbalancer.
		ClusterData ClusterData
		// Provider hold the information (credentials, external templates etc.)
		// that were passed by the user in the InputManifest.
		Provider *spec.Provider
		// Regions hold all the regions from the used provider within a single cluster.
		// Example:
		// If you specify multiple nodepools from the same provider but in different regions
		// and use those nodepool in the same cluster (either K8s or LB) this field will contain
		// those used regions.
		//       - name: gcp-1
		//        providerSpec:
		//          name: gcp-1
		//          region: europe-west1
		//          zone: europe-west1-c
		//        count: 1
		//        serverType: e2-medium
		//        image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
		//
		//      - name: gcp-2
		//        providerSpec:
		//          name: gcp-1
		//          region: europe-west2
		//          zone: europe-west2-a
		//        count: 1
		//        serverType: e2-small
		//        image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
		//        storageDiskSize: 50
		// Regions: ["europe-west2", "europe-west1"].
		Regions []string
	}

	// Networking wraps all data related to generating terraform files for a Provider
	// to set up a common networking infrastructure to be used by all Nodepools from the same Provider.
	// This structure is used when generating template files inside the networking directory
	// of a Template repository.
	Networking struct {
		// ClusterData wraps the identifiers of the current build cluster
		// which can be either K8s or Loadbalancer.
		ClusterData ClusterData
		// Provider hold the information (credentials, external templates etc.)
		// that were passed by the user in the InputManifest.
		Provider *spec.Provider
		// Regions hold all the regions from the used provider within a single cluster.
		// Example:
		// If you specify multiple nodepools from the same provider but in different regions
		// and use those nodepool in the same cluster (either K8s or LB) this field will contain
		// those used regions.
		//       - name: gcp-1
		//        providerSpec:
		//          name: gcp-1
		//          region: europe-west1
		//          zone: europe-west1-c
		//        count: 1
		//        serverType: e2-medium
		//        image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
		//
		//      - name: gcp-2
		//        providerSpec:
		//          name: gcp-1
		//          region: europe-west2
		//          zone: europe-west2-a
		//        count: 1
		//        serverType: e2-small
		//        image: ubuntu-os-cloud/ubuntu-2204-jammy-v20221206
		//        storageDiskSize: 50
		// Regions: ["europe-west2", "europe-west1"].
		Regions []string
		// K8sData contains some additional information that may be needed during the generation of the
		// terraform templates. Such as if A load balancer is attached to the K8s cluster with the ApiServer port.
		// This data will be set if the ClusterType within ClusterData of this object is of type "K8s".
		K8sData K8sData
		// LBData contains some additional information that may be needed during the generation of the
		// terraform templates. Such as all the Roles of the loadbalancer cluster that need to be set
		// for the firewall.
		// This data will be set if the ClusterType within ClusterData of this object is of type "LB".
		LBData LBData
	}

	// Nodepools wraps all data related to generating terraform files to spawn VM instances as described
	// in nodepools from a Provider in the InputManifest. This structure is used when generating template
	// files inside the nodepool directory of a Template repository.
	Nodepools struct {
		// ClusterData wraps the identifiers of the current build cluster
		// which can be either K8s or Loadbalancer.
		ClusterData ClusterData
		// NodePools wraps data specified in the input manifest for a given nodepool and nodes of that nodepool.
		// The nodes are partially initialized by Claudie at first, with only the Name of each respective node
		// being valid. The other details such as PublicIP, PrivateIP will be initialized at a later stage
		// of the pipeline.
		NodePools []NodePoolInfo
	}

	// DNS wraps all data related to generating terraform files to spawn create the specified DNS
	// infrastructure as specified in the InputManifest.
	DNS struct {
		// ClusterName is the name of the Loadbalancer cluster the DNS is to be created for.
		ClusterName string
		// ClusterHash is the hash of the Loadbalancer cluster the DNS is to be created for.
		ClusterHash string
		// Hostname is the hostname specified in the InputManifest.
		Hostname string
		// DNSZone is the zone in which the DNS is to be created as specified in the InputManifest.
		DNSZone string
		// RecordData holds IP addresses of all the nodes of the Loadbalancer cluster the DNS is to be created for.
		RecordData RecordData
		// Provider hold the information (credentials, external templates etc.)
		// that were passed by the user in the InputManifest.
		Provider *spec.Provider
	}
)

// All the following types grouped are passed in as "Outputs" from
// using the generated template files.
type (
	// NodepoolIPs wrap the output data that is acquired from using the
	// generated template files.
	NodepoolIPs struct {
		// IPs holds the IPv4 addresses of the spawned VM instances from the
		// generated templates files. It is expected that the template files
		// in the nodepool directory that spawn the VM instances also
		// expose the IP addresses of the Instances.
		// For example (in the case of our own hetzner template files):
		//
		// output "{{ $nodepool.Name }}_{{ $uniqueFingerPrint }}_{{ $specName }}" {
		//  value = {
		//    {{- range $node := $nodepool.Nodes }}
		//        {{- $serverResourceName := printf "%s_%s" $node.Name $resourceSuffix }}
		//        "${hcloud_server.{{ $serverResourceName }}.name}" = hcloud_server.{{ $serverResourceName }}.ipv4_address
		//    {{- end }}
		//  }
		//}
		IPs map[string]any `json:"-"`
	}

	// DNSDomain wrap the output data that is acquired from using the
	// generated DNS template files.
	DNSDomain struct {
		// Domain holds the fully qualified domain name with which the
		// DNS records were created with. It is expected that the template
		// files in the DNS directory that create the DNS records also expose
		// the Domain name of the endpoint.
		// For example (in the case of our own hetznerdns template files):
		//
		// output "{{ .Data.ClusterName }}-{{ .Data.ClusterHash }}-{{ $uniqueFingerPrint }}_{{ $specName }}" {
		// 	value = { "{{ .Data.ClusterName }}-{{ .Data.ClusterHash }}-endpoint" = format("%s.%s", "{{ .Data.HostnameHash }}", "{{ .Data.DNSZone }}")}
		//}
		Domain map[string]string `json:"-"`
	}
)
