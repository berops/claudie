package usecases

import "github.com/berops/claudie/proto/pb"

const (
	// baseDirectory is ansibler base directory
	baseDirectory = "services/ansibler/server"
	// outputDirectory is directory used to generate ansible playbooks/inventories.
	outputDirectory = "clusters"
	// sshPrivateKeyFileExtension is a private key file extension.
	sshPrivateKeyFileExtension = "pem"
)

type Usecases struct{}

type (
	NodepoolsInfo struct {
		Nodepools      []*pb.NodePool
		PrivateKey     string
		ClusterID      string
		ClusterNetwork string
	}

	AllNodesInventoryData struct {
		NodepoolsInfos []*NodepoolsInfo
	}
)
