package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	scrapeconfig "github.com/berops/claudie/services/kuber/server/domain/utils/scrapeConfig"
)

func (u *Usecases) StoreLbScrapeConfig(ctx context.Context, req *pb.StoreLbScrapeConfigRequest) (*pb.StoreLbScrapeConfigResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(req.Cluster.ClusterInfo))

	clusterID := utils.GetClusterID(req.Cluster.ClusterInfo)
	clusterDir := filepath.Join(outputDir, clusterID)
	logger.Info().Msgf("Storing load balancer scrape-config")

	sc := scrapeconfig.ScrapeConfig{
		Cluster:    req.GetCluster(),
		LBClusters: req.GetDesiredLoadbalancers(),
		Directory:  clusterDir,
	}

	if err := sc.GenerateAndApplyScrapeConfig(); err != nil {
		logger.Err(err).Msgf("Error while applying scrape config for Loadbalancer nodes")
		return nil, fmt.Errorf("error while setting up the loadbalancer scrape-config for %s : %w", clusterID, err)
	}
	logger.Info().Msgf("Load balancer scrape-config successfully set up")

	return &pb.StoreLbScrapeConfigResponse{}, nil
}
