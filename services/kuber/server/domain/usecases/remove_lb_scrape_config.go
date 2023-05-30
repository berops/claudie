package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/lbScrapeConfig"
)

// RemoveLBScrapeConfig deletes the Kubernetes secret containing Prometheus scrape config related to
// the LB clusters attached to given K8s cluster.
// TODO: understand when this will occur.
func (u *Usecases) RemoveLBScrapeConfig(ctx context.Context, request *pb.RemoveLbScrapeConfigRequest) (*pb.RemoveLbScrapeConfigResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.Cluster.ClusterInfo))

	clusterID := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, request.Cluster.ClusterInfo.Hash)
	outputDir := filepath.Join(outputDir, clusterID)

	logger.Info().Msgf("Deleting the Prometheus scrape-config for attached LB clusters")

	prometheusScrapeConfigManagerForLBClusters := &lbScrapeConfig.PrometheusScrapeConfigManagerForLBClusters{
		OutputDirectory: outputDir,

		K8sCluster: request.GetCluster(),
	}
	prometheusScrapeConfigManagerForLBClusters.RemoveScrapeConfig()

	logger.Info().Msgf("Deleted Prometheus scrape-config for attached LB clusters successfully")

	return &pb.RemoveLbScrapeConfigResponse{}, nil
}
