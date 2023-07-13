package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/autoscaler"
)

// DestroyClusterAutoscaler removes deployment of Cluster Autoscaler from the management cluster for given k8s cluster.
func (u *Usecases) DestroyClusterAutoscaler(ctx context.Context, request *pb.DestroyClusterAutoscalerRequest) (*pb.DestroyClusterAutoscalerResponse, error) {
	// Create output dir
	tempClusterID := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, utils.CreateHash(5))
	clusterID := utils.GetClusterID(request.Cluster.ClusterInfo)
	clusterDir := filepath.Join(outputDir, tempClusterID)
	logger := utils.CreateLoggerWithClusterName(clusterID)

	var err error
	// Log success/error message.
	defer func() {
		if err != nil {
			logger.Err(err).Msgf("Error while destroying cluster autoscaler")
		} else {
			logger.Info().Msgf("Cluster autoscaler successfully destroyed")
		}
	}()

	if err = utils.CreateDirectory(clusterDir); err != nil {
		return nil, fmt.Errorf("error while creating directory %s : %w", clusterDir, err)
	}

	// Destroy cluster autoscaler.
	logger.Info().Msgf("Destroying Cluster Autoscaler deployment")
	autoscalerManager := autoscaler.NewAutoscalerManager(request.ProjectName, request.Cluster, clusterDir)
	if err := autoscalerManager.DestroyClusterAutoscaler(); err != nil {
		return nil, fmt.Errorf("error while destroying cluster autoscaler for %s : %w", clusterID, err)
	}

	return &pb.DestroyClusterAutoscalerResponse{}, nil
}
