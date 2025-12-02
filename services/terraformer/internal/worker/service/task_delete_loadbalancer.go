package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

func destroyLoadBalancer(
	logger zerolog.Logger,
	projectName string,
	toDestroy string,
	lbs []*spec.LBclusterV2,
	processLimit *semaphore.Weighted,
	stores Stores,
	tracker Tracker,
) {
	idx := clusters.IndexLoadbalancerByIdV2(toDestroy, lbs)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Update task validation failed, required loadbalancer to delete %q to be present, ignoring", toDestroy)
		return
	}

	lb := loadbalancer.LBcluster{
		ProjectName:       projectName,
		Cluster:           lbs[idx],
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", lb.Cluster.ClusterInfo.Id()).Logger()
	if err := DestroyCluster(buildLogger, projectName, &lb, stores.s3, stores.dynamo); err != nil {
		logger.Err(err).Msg("Failed to destroy load balancer")
		tracker.Diagnostics.Push(err)
		return
	}

	clear := tracker.Result.Clear()
	clear.LoadBalancers(lb.Cluster.ClusterInfo.Id())
	clear.Commit()
}
