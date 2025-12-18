package service

import (
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"
	scrapeconfig "github.com/berops/claudie/services/kuber/internal/worker/service/internal/scrape-config"
	"github.com/rs/zerolog"
)

func StoreScrapeConfig(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Storing scrape config")

	var k8s *spec.K8SclusterV2
	var lbs []*spec.LBclusterV2

	switch do := tracker.Task.Do.(type) {
	case *spec.TaskV2_Create:
		k8s = do.Create.K8S
		lbs = do.Create.LoadBalancers
	case *spec.TaskV2_Update:
		k8s = do.Update.State.K8S
		lbs = do.Update.State.LoadBalancers
	default:
		logger.
			Warn().
			Msgf("Received task %T while wanting to store scrape config, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	var (
		tempClusterId = fmt.Sprintf("%s-%s", k8s.ClusterInfo.Id(), hash.Create(7))
		clusterDir    = filepath.Join(OutputDir, tempClusterId)
		sc            = scrapeconfig.ScrapeConfig{
			Cluster:    k8s,
			LBClusters: lbs,
			Directory:  clusterDir,
		}
	)

	if err := sc.GenerateAndApplyScrapeConfig(); err != nil {
		logger.Err(err).Msg("Failed to apply scrape config")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msgf("Scrape config successfully set up")
}
