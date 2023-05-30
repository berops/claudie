package usecases

import "net"

const outputDir = "services/kuber/server/clusters"

type Usecases struct{}

type (
	ClusterMetadata struct {
		// NodeIps maps node-name to public-private ip pairs.
		NodeIps map[string]IPPair `json:"node_ips"`

		// PrivateKey is the private SSH key for the nodes.
		PrivateKey string `json:"private_key"`
	}

	IPPair struct {
		PublicIP  net.IP `json:"public_ip"`
		PrivateIP net.IP `json:"private_ip"`
	}
)
