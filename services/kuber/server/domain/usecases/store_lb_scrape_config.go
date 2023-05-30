package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils/lbScrapeConfig"
)

// StoreLbScrapeConfig first generates the Prometheus scrape-config for scraping metrics from the
// attached LB clusters. It puts that scrape-config into a Kubernetes secret and applies that secret
// to the given K8s cluster (in the "monitoring" namespace).
func (u *Usecases) StoreLbScrapeConfig(ctx context.Context, request *pb.StoreLbScrapeConfigRequest) (*pb.StoreLbScrapeConfigResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.Cluster.ClusterInfo))

	clusterID := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, request.Cluster.ClusterInfo.Hash)
	outputDir := filepath.Join(outputDir, clusterID)

	logger.Info().Msgf("Generating and applying Prometheus scrape-config for LB clusters")

	prometheusScrapeConfigManagerForLBClusters := &lbScrapeConfig.PrometheusScrapeConfigManagerForLBClusters{
		OutputDirectory: outputDir,

		K8sCluster:         request.GetCluster(),
		AttachedLBClusters: request.GetDesiredLoadbalancers(),
	}
	if err := prometheusScrapeConfigManagerForLBClusters.GenerateAndApplyScrapeConfig(); err != nil {
		return nil, fmt.Errorf("error while setting up the loadbalancer scrape-config for %s : %w", clusterID, err)
	}

	logger.Info().Msgf("Prometheus scrape-config for LB clusters has successfully been applied")

	return &pb.StoreLbScrapeConfigResponse{}, nil
}
