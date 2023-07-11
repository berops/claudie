package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/autoscaler"
)

func (u *Usecases) SetUpClusterAutoscaler(ctx context.Context, request *pb.SetUpClusterAutoscalerRequest) (*pb.SetUpClusterAutoscalerResponse, error) {
	// Create output dir
	clusterID := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, utils.CreateHash(5))
	clusterDir := filepath.Join(outputDir, clusterID)
	if err := utils.CreateDirectory(clusterDir); err != nil {
		return nil, fmt.Errorf("error while creating directory %s : %w", clusterDir, err)
	}

	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.Cluster.ClusterInfo))

	// Set up cluster autoscaler.
	autoscalerBuilder := autoscaler.NewAutoscalerBuilder(request.ProjectName, request.Cluster, clusterDir)
	if err := autoscalerBuilder.SetUpClusterAutoscaler(); err != nil {
		logger.Err(err).Msgf("Error while setting up cluster autoscaler")
		return nil, fmt.Errorf("error while setting up cluster autoscaler for %s : %w", request.Cluster.ClusterInfo.Name, err)
	}

	logger.Info().Msgf("Cluster autoscaler successfully set up")
	return &pb.SetUpClusterAutoscalerResponse{}, nil
}
