package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/cluster"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/autoscaler"
)

// DestroyClusterAutoscaler removes deployment of Cluster Autoscaler from the management cluster for given k8s cluster.
func (u *Usecases) DestroyClusterAutoscaler(ctx context.Context, request *pb.DestroyClusterAutoscalerRequest) (*pb.DestroyClusterAutoscalerResponse, error) {
	logger := loggerutils.WithClusterName(cluster.Id(request.Cluster.ClusterInfo))

	var err error
	// Log success/error message.
	defer func() {
		if err != nil {
			logger.Err(err).Msgf("Error while destroying cluster autoscaler")
		} else {
			logger.Info().Msgf("Cluster autoscaler successfully destroyed")
		}
	}()

	// Create output dir
	tempClusterID := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, utils.CreateHash(5))
	clusterDir := filepath.Join(outputDir, tempClusterID)
	if err = utils.CreateDirectory(clusterDir); err != nil {
		return nil, fmt.Errorf("error while creating directory %s : %w", clusterDir, err)
	}

	// Destroy cluster autoscaler.
	logger.Info().Msgf("Destroying Cluster Autoscaler deployment")
	autoscalerManager := autoscaler.NewAutoscalerManager(request.ProjectName, request.Cluster, clusterDir)
	if err := autoscalerManager.DestroyClusterAutoscaler(); err != nil {
		logger.Debug().Msgf("Ignoring Destroy Autoscaler error: %v", err.Error())
	}

	return &pb.DestroyClusterAutoscalerResponse{}, nil
}
