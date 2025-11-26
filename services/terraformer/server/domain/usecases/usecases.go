package usecases

import (
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/services/terraformer/server/domain/ports"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

// SpawnProcessLimit is the number of processes concurrently executing tofu.
var SpawnProcessLimit = envs.GetOrDefaultInt("TERRAFORMER_CONCURRENT_CLUSTERS", 7)

type Usecases struct {
	// Minio connector.
	StateStorage ports.StateStoragePort
	// SpawnProcessLimit limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
}

type Cluster interface {
	// Build builds the cluster.
	Build(logger zerolog.Logger) error
	// Destroy destroys the cluster.
	Destroy(logger zerolog.Logger) error
	// Id returns a cluster ID for the cluster.
	Id() string
	// UpdateCurrentState sets the current state equal to the desired state.
	UpdateCurrentState()
}
