package ports

import (
	"context"
)

type DynamoDBPort interface {
	// DeleteLockFile removes lock file for the terraform state file from dynamoDB.
	DeleteLockFile(ctx context.Context, projectName, clusterId string, keyFormat string) error
}
