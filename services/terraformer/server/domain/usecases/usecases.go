package usecases

import "github.com/berops/claudie/services/terraformer/server/domain/ports"

type Usecases struct {
	DynamoDB ports.DynamoDBPort
	MinIO    ports.MinIOPort
}

type Cluster interface {
	Build() error
	Destroy() error
	Id() string
}
