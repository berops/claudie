package usecases

import (
	"github.com/berops/claudie/internal/envs"
	"golang.org/x/sync/semaphore"
)

// SpawnProcessLimit is the number of processes concurrently executing kubeone.
var SpawnProcessLimit = envs.GetOrDefaultInt("KUBE_ELEVEN_CONCURRENT_CLUSTERS", 7)

type Usecases struct {
	// SpawnProcessLimit limits the number of spawned terraform processes.
	SpawnProcessLimit *semaphore.Weighted
}
