package templates

import (
	"github.com/berops/claudie/proto/pb"
)

type (
	// Nodepools Terraform files input.
	ClusterData struct {
		ClusterName string
		ClusterHash string
		ClusterType string
	}

	ProviderData struct {
		ClusterData ClusterData
		Provider    *pb.Provider
		Regions     []string
		Metadata    map[string]any
	}

	NodePoolInfo struct {
		NodePool  *pb.DynamicNodePool
		Name      string
		Nodes     []*pb.Node
		IsControl bool
	}

	NodepoolsData struct {
		ClusterData ClusterData
		NodePools   []NodePoolInfo
		Metadata    map[string]any
	}

	// DNS Terraform files input
	DNSData struct {
		ClusterName  string
		ClusterHash  string
		HostnameHash string
		DNSZone      string
		NodeIPs      []string
		Provider     *pb.Provider
	}

	fingerPrintedData struct {
		// Data is data passed to the template generator.
		Data any
		// Fingerprint is the checksum of the templates of a given nodepool.
		Fingerprint string
	}
)

type (
	// Nodepool Terraform files output.
	NodepoolIPs struct {
		IPs map[string]any `json:"-"`
	}

	// DNS Terraform files output.
	DNSDomain struct {
		Domain map[string]string `json:"-"`
	}
)
