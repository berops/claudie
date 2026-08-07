package service

import (
	"context"
	"errors"

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
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	action DeleteLoadBalancerNodes,
	tracker Tracker,
) {
	logger.Info().Msg("Deleting LoadBalancer Nodes")

	idx := clusters.IndexLoadbalancerById(action.Delete.Handle, action.State.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't delete nodes for loadbalancer %q that is missing from the received state", action.Delete.Handle)
		return
	}

	var (
		ctx     = context.Background()
		current = action.State.LoadBalancers[idx]
		lb      = loadbalancer.LBcluster{
			ProjectName:       projectName,
			Cluster:           current,
			SpawnProcessLimit: processLimit,
		}
	)

	logger = logger.With().Str("cluster", lb.Id()).Logger()

	if action.Delete.WithNodePool {
		deleted := nodepools.FindByName(action.Delete.Nodepool, current.ClusterInfo.NodePools)
		current.ClusterInfo.NodePools = nodepools.DeleteByName(current.ClusterInfo.NodePools, action.Delete.Nodepool)

		if err := lb.DestroyNodePool(ctx, logger, deleted, stores.s3); err != nil {
			if !errors.Is(err, loadbalancer.ErrReconcileAll) {
				logger.Err(err).Msgf("Failed to destroy nodepool %q", action.Delete.Nodepool)
				tracker.Diagnostics.Push(err)
				return
			}

			logger.
				Warn().
				Msgf("Handling reconcile all, after %q nodepool deletion", action.Delete.Nodepool)

			if err := lb.ReconcileAll(ctx, logger); err != nil {
				logger.Err(err).Msgf("Failed reconcile infra after nodepool %q destruction", action.Delete.Nodepool)
				tracker.Diagnostics.Push(err)
				return
			}
		}
	} else {
		np := nodepools.FindByName(action.Delete.Nodepool, current.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf(
					"Can't delete nodes from nodepool %q as the nodepool is missing from the received state",
					action.Delete.Nodepool,
				)
			return
		}

		nodepools.DeleteNodes(np, action.Delete.Nodes)
		if err := lb.ReconcileNodePool(ctx, logger, action.Delete.Nodepool, loadbalancer.ReconcileModeRead); err != nil {
			if !errors.Is(err, loadbalancer.ErrReconcileAll) {
				logger.Err(err).Msgf("Failed to destroy nodes in nodepool %q", action.Delete.Nodepool)
				tracker.Diagnostics.Push(err)
				return
			}

			logger.
				Warn().
				Msgf("Handling reconcile all, after node deletion from %q", action.Delete.Nodepool)

			if err := lb.ReconcileAll(ctx, logger); err != nil {
				logger.Err(err).Msgf("Failed to reconcile infra after destruction of nodes in nodepool %q", action.Delete.Nodepool)
				tracker.Diagnostics.Push(err)
				return
			}
		}
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}
