package ports

import "context"

type MinIOPort interface {
	DeleteTfStateFile(ctx context.Context, projectName, clusterId string, keyFormat string) error
}
