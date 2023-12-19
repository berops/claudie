package usecases

import "net"

const (
	outputDir = "services/kuber/server/clusters"
)

type Usecases struct{}

type (
	// IPPair contains pair of public and private IP for a single node.
	IPPair struct {
		// PublicIP describes public IP.
		PublicIP net.IP `json:"public_ip"`
		// PrivateIP describes private IP.
		PrivateIP net.IP `json:"private_ip"`
	}

	// StaticNodeInfo contains metadata info about static node.
	StaticNodeInfo struct {
		// Endpoint is an endpoint for static nodes in the static node pool.
		Endpoint string `json:"endpoint"`
		// PrivateKey is the private SSH key for the node.
		PrivateKey string `json:"node_private_key"`
	}

	// ClusterMetadata contains metadata for the whole cluster. This metadata will be exported as a secret in management cluster.
	ClusterMetadata struct {
		// DynamicNodepools contains metadata for dynamic nodepools.
		DynamicNodepools DynamicNodepool `json:"dynamic_nodepools"`
		// PrivateKey is the private SSH key for the dynamic nodes.
		PrivateKey string `json:"cluster_private_key"`
		// StaticNodepools contains metadata for static nodepools.
		StaticNodepools StaticNodepool `json:"static_nodepools"`
		// LoadBalancerNodePools contain metadata for lb nodepools.
		LoadBalancerNodePools map[string]LoadBalancerNodePools `json:"load_balancer_node_pools"`
	}

	// DynamicNodepool contains map of node names and their IP pair.
	DynamicNodepool struct {
		// NodeIps maps node-name to public-private ip pairs for dynamic node pools.
		NodeIps map[string]IPPair `json:"node_ips"`
	}

	// StaticNodepool contains map of node names and their static metadata.
	StaticNodepool struct {
		// NodeIps maps node-name to endpoint-key pairs for static node pools.
		NodeInfo map[string]StaticNodeInfo `json:"node_info"`
	}

	LoadBalancerNodePools struct {
		// NodeIps maps node-name to public-private ip pairs for dynamic node pools.
		NodeIps map[string]IPPair `json:"node_ips"`
		// PrivateKey is the private SSH key for the dynamic nodes.
		PrivateKey string `json:"cluster_private_key"`
	}
)
