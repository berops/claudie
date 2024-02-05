package usecases

import (
	"github.com/berops/claudie/services/terraformer/server/domain/ports"
	"github.com/rs/zerolog"
)

const (
	// SpawnProcessLimit is the number of processes concurrently executing terraform.
	SpawnProcessLimit = 5
)

type Usecases struct {
	// DynamoDB connector.
	DynamoDB ports.DynamoDBPort
	// Minio connector.
	StateStorage ports.StateStoragePort
	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned terraform
	// processes. This values should always be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
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
