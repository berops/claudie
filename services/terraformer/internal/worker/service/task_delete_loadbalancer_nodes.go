package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

type DeleteLoadBalancerNodes struct {
	State  *spec.Update_State
	Delete *spec.Update_TerraformerDeleteLoadBalancerNodes
}

func deleteLoadBalancerNodes(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	action DeleteLoadBalancerNodes,
	tracker Tracker,
) {
	// Currently there is no special mechanism for deleting the
	// nodes of the loadbalancer as the whole cluster shares a
	// single state file, thus simply just remove the node from
	// the state and reconcile the cluster.
	idx := clusters.IndexLoadbalancerById(action.Delete.Handle, action.State.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't delete nodes for loadbalancer %q that is missing from the received state", action.Delete.Handle)
		return
	}

	current := action.State.LoadBalancers[idx]

	if action.Delete.WithNodePool {
		// deleting whole nodepool, if the nodepool is not found there are no side-effects.
		current.ClusterInfo.NodePools = nodepools.DeleteByName(current.ClusterInfo.NodePools, action.Delete.Nodepool)
	} else {
		np := nodepools.FindByName(action.Delete.Nodepool, current.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf(
					"Can't delete nodes from nodepool %q of loadbalancer %q as the nodepool is missing form the received state",
					action.Delete.Nodepool,
					action.Delete.Handle,
				)
			return
		}
		nodepools.DeleteNodes(np, action.Delete.Nodes)
	}

	lb := loadbalancer.LBcluster{
		ProjectName:       projectName,
		Cluster:           current,
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", lb.Id()).Logger()
	if err := BuildLoadbalancers(buildLogger, lb); err != nil {
		buildLogger.Err(err).Msg("Failed to reconcile cluster after nodes deletion")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}
