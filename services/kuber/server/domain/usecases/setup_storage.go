package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/longhorn"
)

// SetUpStorage installs and configures Longhorn in the given K8s cluster.
// (Installation of Longhorn prerequisites has already been taken care in the ansibler microservice.)
func (u *Usecases) SetUpStorage(ctx context.Context, request *pb.SetUpStorageRequest) (*pb.SetUpStorageResponse, error) {
	id := request.DesiredCluster.ClusterInfo.Id()
	logger := loggerutils.WithClusterName(id)

	clusterDir := filepath.Join(outputDir, id)

	logger.Info().Msgf("Setting up the longhorn")
	longhorn := longhorn.Longhorn{Cluster: request.DesiredCluster, Directory: clusterDir}
	if err := longhorn.SetUp(); err != nil {
		logger.Err(err).Msgf("Error while setting up the longhorn")
		return nil, fmt.Errorf("error while setting up the longhorn for %s : %w", id, err)
	}
	logger.Info().Msgf("Longhorn successfully set up")

	return &pb.SetUpStorageResponse{DesiredCluster: request.DesiredCluster}, nil
}
