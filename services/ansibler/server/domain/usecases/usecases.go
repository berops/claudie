package usecases

import "github.com/berops/claudie/services/ansibler/server/utils"

const (
	// baseDirectory is ansibler base directory
	baseDirectory = "services/ansibler/server"
	// outputDirectory is directory used to generate ansible playbooks/inventories.
	outputDirectory = "clusters"
	// sshPrivateKeyFileExtension is a private key file extension.
	sshPrivateKeyFileExtension = "pem"
	// SpawnProcessLimit is the number of processes concurrently executing ansible.
	SpawnProcessLimit = 5
)

type Usecases struct {
	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned ansible
	// processes. This values should always be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
}

type (
	NodepoolsInfo struct {
		Nodepools      utils.NodePools
		PrivateKey     string
		ClusterID      string
		ClusterNetwork string
	}

	AllNodesInventoryData struct {
		NodepoolsInfo []*NodepoolsInfo
	}
)
