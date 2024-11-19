package usecases

import (
	"github.com/berops/claudie/services/ansibler/server/utils"

	"golang.org/x/sync/semaphore"
)

const (
	// baseDirectory is ansibler base directory
	baseDirectory = "services/ansibler/server"
	// outputDirectory is directory used to generate ansible playbooks/inventories.
	outputDirectory = "clusters"
	// SpawnProcessLimit is the number of processes concurrently executing ansible.
	SpawnProcessLimit = 5
)

type Usecases struct {
	// SpawnProcessLimit limits the number of spawned ansible processes.
	SpawnProcessLimit *semaphore.Weighted
}

type (
	NodepoolsInfo struct {
		Nodepools      utils.NodePools
		ClusterID      string
		ClusterNetwork string
	}

	AllNodesInventoryData struct {
		NodepoolsInfo []*NodepoolsInfo
	}
)
