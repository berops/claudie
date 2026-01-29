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

func SetUpClusterAutoscaler(logger zerolog.Logger, manifestName string, clusters *spec.Clusters) error {
	if envs.Namespace == "" {
		return nil
	}

	var (
		clusterID         = clusters.K8S.ClusterInfo.Id()
		tempClusterID     = fmt.Sprintf("%s-%s", clusterID, hash.Create(5))
		clusterDir        = filepath.Join(outputDir, tempClusterID)
		autoscalerManager = autoscaler.NewAutoscalerManager(manifestName, clusters.K8S, clusterDir)
	)

	// Create output dir
	if err := fileutils.CreateDirectory(clusterDir); err != nil {
		return fmt.Errorf("error while creating directory: %w", err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			logger.Err(err).Msgf("Failed to remove directory: %s", clusterDir)
		}
	}()

	if err := autoscalerManager.SetUpClusterAutoscaler(logger); err != nil {
		return fmt.Errorf("error while setting up cluster autoscaler: %w", err)
	}

	return nil
}
