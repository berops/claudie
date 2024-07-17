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
		DynamicNodepools map[string]DynamicNodepool `json:"dynamic_nodepools"`
		// StaticNodepools contains metadata for static nodepools.
		StaticNodepools map[string]StaticNodepool `json:"static_nodepools"`
		// DynamicLoadBalancerNodePools contain metadata for dynamic lb nodepools.
		DynamicLoadBalancerNodePools map[string]map[string]DynamicNodepool `json:"dynamic_load_balancer_nodepools"`
		// StaticLoadBalancerNodePools contain metadata for static lb nodepools.
		StaticLoadBalancerNodePools map[string]map[string]StaticNodepool `json:"static_load_balancer_nodepools"`
	}

	// DynamicNodepool contains map of node names and their IP pair.
	DynamicNodepool struct {
		// NodeIps maps node-name to public-private ip pairs for dynamic node pools.
		NodeIps map[string]IPPair `json:"node_ips"`
		// PrivateKey is the private SSH key for the dynamic nodes.
		PrivateKey string `json:"nodepool_private_key"`
	}

	// StaticNodepool contains map of node names and their static metadata.
	StaticNodepool struct {
		// NodeIps maps node-name to endpoint-key pairs for static node pools.
		NodeInfo map[string]StaticNodeInfo `json:"node_info"`
	}
)
