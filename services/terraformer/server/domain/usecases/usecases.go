package usecases

import (
	"github.com/rs/zerolog"

	"github.com/berops/claudie/services/terraformer/server/domain/ports"
)

type Usecases struct {
	// DynamoDB connector.
	DynamoDB ports.DynamoDBPort
	// Minio connector.
	MinIO ports.MinIOPort
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
