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
	tempClusterID := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, utils.CreateHash(5))
	clusterID := utils.GetClusterID(request.Cluster.ClusterInfo)
	clusterDir := filepath.Join(outputDir, tempClusterID)
	logger := utils.CreateLoggerWithClusterName(clusterID)
	var err error
	// Log success/error message.
	defer func() {
		if err != nil {
			logger.Err(err).Msgf("Error while setting up cluster autoscaler")
		} else {
			logger.Info().Msgf("Cluster autoscaler successfully set up")
		}
	}()

	if err := utils.CreateDirectory(clusterDir); err != nil {
		return nil, fmt.Errorf("error while creating directory %s : %w", clusterDir, err)
	}

	// Set up cluster autoscaler.
	autoscalerManager := autoscaler.NewAutoscalerManager(request.ProjectName, request.Cluster, clusterDir)
	if err := autoscalerManager.SetUpClusterAutoscaler(); err != nil {
		return nil, fmt.Errorf("error while setting up cluster autoscaler for %s : %w", clusterID, err)
	}
	return &pb.SetUpClusterAutoscalerResponse{}, nil
}
