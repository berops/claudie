package usecases

import (
	"github.com/berops/claudie/services/terraformer/server/domain/ports"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

const (
	// SpawnProcessLimit is the number of processes concurrently executing tofu.
	SpawnProcessLimit = 5
)

type Usecases struct {
	// DynamoDB connector.
	DynamoDB ports.DynamoDBPort
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
