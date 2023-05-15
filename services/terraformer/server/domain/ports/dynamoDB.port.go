package ports

import (
	"context"
)

type DynamoDBPort interface {
	DeleteTfStateLockFile(ctx context.Context, projectName, clusterId string, isForDNS bool) error
}
