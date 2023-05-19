package usecases

import "github.com/berops/claudie/proto/pb"

const (
	baseDirectory   = "services/ansibler/server"
	outputDirectory = "clusters"

	sshPrivateKeyFileExtension = "pem"

	// Name of the generated Ansible inventory file.
	inventoryFileName = "inventory.ini"
	// Name of the generated Ansible inventory file (for LB cluster).
	lbInventoryFileName = "lb-inventory.goini"

	allNodes_InventoryTemplateFileName = "all-node-inventory.goini"
)

type Usecases struct{}

type (
	// By Cluster here we mean the desired version of the cluster.
	NodepoolsInfoOfCluster struct {
		Nodepools []*pb.NodePool

		PrivateKey     string
		ClusterId      string
		ClusterNetwork string
	}

	AllNodesInventoryData struct {
		NodepoolsInfos []*NodepoolsInfoOfCluster
	}

	LbInventoryData struct {
		K8sNodepools []*pb.NodePool
		LBClusters   []*pb.LBcluster
		ClusterID    string
	}
)
