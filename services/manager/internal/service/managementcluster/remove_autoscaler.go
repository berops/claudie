package managementcluster

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/service/managementcluster/internal/autoscaler"
	"github.com/rs/zerolog"
)

// DestroyClusterAutoscaler removes deployment of Cluster Autoscaler from the management cluster for given k8s cluster.
func DestroyClusterAutoscaler(logger zerolog.Logger, manifestName string, clusters *spec.Clusters) error {
	if envs.Namespace == "" {
		return nil
	}

	var (
		clusterID         = clusters.K8S.ClusterInfo.Id()
		tempClusterID     = fmt.Sprintf("%s-%s", clusterID, hash.Create(5))
		clusterDir        = filepath.Join(outputDir, tempClusterID)
		autoscalerManager = autoscaler.NewAutoscalerManager(manifestName, clusters.K8S, clusterDir)
	)

	if err := fileutils.CreateDirectory(clusterDir); err != nil {
		return fmt.Errorf("error while creating directory %s : %w", clusterDir, err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			logger.Err(err).Msgf("Failed to remove directory: %s", clusterDir)
		}
	}()

	if err := autoscalerManager.DestroyClusterAutoscaler(logger); err != nil {
		return fmt.Errorf("error while destroying cluster autoscaler: %w", err)
	}

	return nil
}
