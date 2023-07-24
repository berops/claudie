package ports

import "context"

type MinIOPort interface {
	// DeleteStateFile removes terraform state file from MinIO.
	DeleteStateFile(ctx context.Context, projectName, clusterId string, keyFormat string) error
	// Stat checks whether the object exists.
	Stat(ctx context.Context, projectName, clusterId, keyFormat string) error
}
