package usecases

import "github.com/berops/claudie/proto/pb"

const (
	baseDirectory   = "services/ansibler/server"
	outputDirectory = "clusters"

	sshPrivateKeyFileExtension = "pem"

	// Name of the generated Ansible inventory file.
	inventoryFileName = "inventory.ini"

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
)
