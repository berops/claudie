package usecases

import (
	"github.com/rs/zerolog"

	"github.com/berops/claudie/services/terraformer/server/domain/ports"
)

type Usecases struct {
	DynamoDB ports.DynamoDBPort
	MinIO    ports.MinIOPort
}

type Cluster interface {
	Build(logger zerolog.Logger) error
	Destroy(logger zerolog.Logger) error
	Id() string
}
