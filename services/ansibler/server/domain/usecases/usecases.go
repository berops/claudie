package usecases

import "github.com/berops/claudie/proto/pb"

const (
	// baseDirectory is ansibler base directory
	baseDirectory = "services/ansibler/server"
	// outputDirectory is directory used to generate ansible playbooks/inventories.
	outputDirectory = "clusters"
	// sshPrivateKeyFileExtension is a private key file extension.
	sshPrivateKeyFileExtension = "pem"
	// inventoryFileName is name of the generated Ansible inventory file.
	inventoryFileName = "inventory.ini"
	//allNodesInventoryTemplateFileName is name of the generated Ansible inventory file for all nodes.
	allNodesInventoryTemplateFileName = "all-node-inventory.goini"
)

type Usecases struct{}

type (
	// By Cluster here we mean the desired version of the cluster.
	NodepoolsInfoOfCluster struct {
		Nodepools      []*pb.NodePool
		PrivateKey     string
		ClusterId      string
		ClusterNetwork string
	}

	AllNodesInventoryData struct {
		NodepoolsInfos []*NodepoolsInfoOfCluster
	}
)
