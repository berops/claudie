package service

import (
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"
	scrapeconfig "github.com/berops/claudie/services/kuber/internal/worker/service/internal/scrape-config"
	"github.com/rs/zerolog"
)

func RemoveScrapeConfig(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Deleting scrape-config")

	update, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task %T while wanting to remove scrape config, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	var (
		tempClusterId = fmt.Sprintf("%s-%s", update.Update.State.K8S.ClusterInfo.Id(), hash.Create(7))
		clusterDir    = filepath.Join(OutputDir, tempClusterId)
		sc            = scrapeconfig.ScrapeConfig{
			Cluster:    update.Update.State.K8S,
			LBClusters: nil, // not needed for removal.
			Directory:  clusterDir,
		}
	)

	if err := sc.RemoveLBScrapeConfig(); err != nil {
		logger.Err(err).Msg("Failed to remove scrape config")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Scrape config successfully deleted")
}
