package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	scrapeconfig "github.com/berops/claudie/services/kuber/server/domain/utils/scrape-config"
)

func (u *Usecases) StoreLbScrapeConfig(ctx context.Context, req *pb.StoreLbScrapeConfigRequest) (*pb.StoreLbScrapeConfigResponse, error) {
	clusterID := utils.GetClusterID(req.Cluster.ClusterInfo)
	logger := utils.CreateLoggerWithClusterName(clusterID)

	clusterDir := filepath.Join(outputDir, clusterID)
	logger.Info().Msgf("Storing loadbalancer scrape-config")

	sc := scrapeconfig.ScrapeConfig{
		Cluster:    req.GetCluster(),
		LBClusters: req.GetDesiredLoadbalancers(),
		Directory:  clusterDir,
	}

	if err := sc.GenerateAndApplyScrapeConfig(); err != nil {
		logger.Err(err).Msgf("Error while applying scrape config for Loadbalancers")
		return nil, fmt.Errorf("error while setting up the loadbalancer scrape-config for %s : %w", clusterID, err)
	}
	logger.Info().Msgf("Loadbalancer scrape-config successfully set up")

	return &pb.StoreLbScrapeConfigResponse{}, nil
}
