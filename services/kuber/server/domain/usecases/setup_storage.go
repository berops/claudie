package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/longhorn"
)

// SetUpStorage installs and configures Longhorn in the given K8s cluster.
// (Installation of Longhorn prerequisites has already been taken care in the ansibler microservice.)
func (u *Usecases) SetUpStorage(ctx context.Context, request *pb.SetUpStorageRequest) (*pb.SetUpStorageResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.DesiredCluster.ClusterInfo))

	var (
		clusterID = fmt.Sprintf("%s-%s", request.DesiredCluster.ClusterInfo.Name, request.DesiredCluster.ClusterInfo.Hash)
		outputDir = filepath.Join(outputDir, clusterID)
	)

	logger.Info().Msgf("Setting up the Longhorn")
	longhorn := longhorn.Longhorn{Cluster: request.DesiredCluster, OutputDirectory: outputDir}
	if err := longhorn.SetUp(); err != nil {
		return nil, fmt.Errorf("error while setting up the Longhorn for %s : %w", clusterID, err)
	}

	logger.Info().Msgf("Longhorn successfully set up")

	return &pb.SetUpStorageResponse{DesiredCluster: request.DesiredCluster}, nil
}
