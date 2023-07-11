package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	scrapeconfig "github.com/berops/claudie/services/kuber/server/domain/utils/scrapeConfig"
)

// RemoveLBScrapeConfig deletes the Kubernetes secret containing Prometheus scrape config related to
// the LB clusters attached to given K8s cluster.
func (u *Usecases) RemoveLBScrapeConfig(ctx context.Context, request *pb.RemoveLbScrapeConfigRequest) (*pb.RemoveLbScrapeConfigResponse, error) {
	tempClusterID := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, utils.CreateHash(5))
	clusterID := utils.GetClusterID(request.Cluster.ClusterInfo)
	clusterDir := filepath.Join(outputDir, tempClusterID)

	logger := utils.CreateLoggerWithClusterName(clusterID)

	logger.Info().Msgf("Deleting load balancer scrape-config")
	sc := scrapeconfig.ScrapeConfig{
		Cluster:   request.GetCluster(),
		Directory: clusterDir,
	}

	if err := sc.RemoveLbScrapeConfig(); err != nil {
		logger.Err(err).Msgf("Error while removing scrape config for Loadbalancer nodes")
		return nil, fmt.Errorf("error while removing old loadbalancer scrape-config for %s : %w", clusterID, err)
	}
	logger.Info().Msgf("Load balancer scrape-config successfully deleted")

	return &pb.RemoveLbScrapeConfigResponse{}, nil
}
