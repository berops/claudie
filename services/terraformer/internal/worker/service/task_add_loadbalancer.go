package service

import (
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

func addLoadBalancer(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	toReconcile *spec.LBcluster,
	tracker Tracker,
) {
	logger.Info().Msg("Add LoadBalancer")

	lb := loadbalancer.LBcluster{
		ProjectName:       projectName,
		Cluster:           toReconcile,
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", lb.Cluster.ClusterInfo.Id()).Logger()
	if err := BuildLoadbalancers(buildLogger, lb); err != nil {
		buildLogger.Err(err).Msg("Failed to reconcile loadbalancer")
		tracker.Diagnostics.Push(err)
		// Some part of the loadbalancer infrastructure was not build successfully.
		// Since we still want to report the partially build infrastructure back to the
		// caller, fallthrough here.
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}
