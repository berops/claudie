package ports

import (
	"context"
)

type DynamoDBPort interface {
	// DeleteLockFile removes lock file for the OpenTofu state file from dynamoDB.
	DeleteLockFile(ctx context.Context, projectName, clusterId string, keyFormat string) error
}
