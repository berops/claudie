package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	scrapeconfig "github.com/berops/claudie/services/kuber/server/domain/utils/scrape-config"
)

// RemoveLBScrapeConfig deletes the Kubernetes secret containing Prometheus scrape config related to
// the LB clusters attached to given K8s cluster.
func (u *Usecases) RemoveLBScrapeConfig(ctx context.Context, request *pb.RemoveLBScrapeConfigRequest) (*pb.RemoveLBScrapeConfigResponse, error) {
	id := request.Cluster.ClusterInfo.Id()
	logger := loggerutils.WithClusterName(id)

	logger.Info().Msgf("Deleting load balancer scrape-config")

	tempClusterID := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, utils.CreateHash(5))
	clusterDir := filepath.Join(outputDir, tempClusterID)

	sc := scrapeconfig.ScrapeConfig{
		Cluster:   request.Cluster,
		Directory: clusterDir,
	}

	if err := sc.RemoveLBScrapeConfig(); err != nil {
		logger.Err(err).Msgf("Error while removing scrape config for Loadbalancer nodes")
		return nil, fmt.Errorf("error while removing old loadbalancer scrape-config for %s : %w", id, err)
	}
	logger.Info().Msgf("Load balancer scrape-config successfully deleted")

	return &pb.RemoveLBScrapeConfigResponse{}, nil
}
