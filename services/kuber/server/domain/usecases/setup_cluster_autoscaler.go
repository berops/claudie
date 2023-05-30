package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/clusterAutoscaler"
)

func (u *Usecases) SetUpClusterAutoscaler(ctx context.Context, request *pb.SetUpClusterAutoscalerRequest) (*pb.SetUpClusterAutoscalerResponse, error) {
	var (
		clusterID = fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, utils.CreateHash(5))
		outputDir = filepath.Join(outputDir, clusterID)
	)
	if err := utils.CreateDirectory(outputDir); err != nil {
		return nil, fmt.Errorf("error while creating directory %s : %w", outputDir, err)
	}

	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.Cluster.ClusterInfo))

	// Set up cluster autoscaler.
	autoscalerBuilder := clusterAutoscaler.NewAutoscalerBuilder(request.ProjectName, request.Cluster, outputDir)
	if err := autoscalerBuilder.SetupClusterAutoscaler(); err != nil {
		logger.Err(err).Msgf("Error while setting up cluster autoscaler")
		return nil, fmt.Errorf("error while setting up cluster autoscaler for %s : %w", request.Cluster.ClusterInfo.Name, err)
	}

	logger.Info().Msgf("Cluster autoscaler successfully set up")
	return &pb.SetUpClusterAutoscalerResponse{}, nil
}
