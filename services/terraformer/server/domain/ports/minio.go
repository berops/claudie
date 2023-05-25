package ports

import "context"

type MinIOPort interface {
	// DeleteStateFile removes terraform state file from MinIO.
	DeleteStateFile(ctx context.Context, projectName, clusterId string, keyFormat string) error
}
