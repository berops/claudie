package templates

import (
	"github.com/berops/claudie/proto/pb"
)

// TODO: cleanup
// helper structs containing only necessary information
// for the templates to be used.
type (
	K8sData struct{ HasAPIServer bool }
	LBData  struct{ Roles []*pb.Role }
	IPData  struct{ V4, EscapedV4 string }

	RecordData struct {
		IP []IPData
	}

	ClusterData struct {
		ClusterName string
		ClusterHash string
		ClusterType string
	}

	NodePoolInfo struct {
		Name      string
		Details   *pb.DynamicNodePool
		Nodes     []*pb.Node
		IsControl bool
	}
)

// Terraform files input.
type (
	ProviderData struct {
		ClusterData ClusterData
		Provider    *pb.Provider
		Regions     []string
	}

	NetworkingData struct {
		ClusterData ClusterData
		Provider    *pb.Provider
		Regions     []string
		K8sData     K8sData
		LBData      LBData
	}

	NodepoolsData struct {
		ClusterData ClusterData
		NodePools   []NodePoolInfo
	}

	DNSData struct {
		ClusterName  string
		ClusterHash  string
		HostnameHash string
		DNSZone      string
		RecordData   RecordData
		Provider     *pb.Provider
	}

	fingerPrintedData struct {
		// Data is data passed to the template generator (one of the above).
		Data any
		// Fingerprint is the checksum of the templates of a given nodepool.
		Fingerprint string
	}
)

// Terraform files output.
type (
	NodepoolIPs struct {
		IPs map[string]any `json:"-"`
	}

	// DNS Terraform files output.
	DNSDomain struct {
		Domain map[string]string `json:"-"`
	}
)
