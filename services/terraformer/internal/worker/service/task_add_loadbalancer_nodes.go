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

type AddLoadBalancerNodes struct {
	State *spec.Update_State
	Add   *spec.Update_TerraformerAddLoadBalancerNodes
}

func addLoadBalancerNodes(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	action AddLoadBalancerNodes,
	tracker Tracker,
) {
	logger.Info().Msg("Adding LoadBalancer Nodes")

	idx := clusters.IndexLoadbalancerById(action.Add.Handle, action.State.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't add nodes for loadbalancer %q that is missing from the received state", action.Add.Handle)
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

	switch kind := action.Add.Kind.(type) {
	case *spec.Update_TerraformerAddLoadBalancerNodes_Existing_:
		np := nodepools.FindByName(kind.Existing.Nodepool, current.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf(
					"Can't add nodes to nodepool %q as the nodepool is missing from the received state",
					kind.Existing.Nodepool,
				)
			return
		}

		// Append new nodes only for dynamic nodepools.
		//
		// Static nodepools node additions are not handled
		// by terraformer at all.
		nodepools.AppendDynamicNodes(np, kind.Existing.Nodes)
		if err := lb.ReconcileNodePool(ctx, logger, kind.Existing.Nodepool, loadbalancer.ReconcileModeRead); err != nil {
			if errors.Is(err, loadbalancer.ErrReconcileAll) {
				err = lb.ReconcileAll(ctx, logger)
			}

			if err != nil {
				logger.Err(err).Msgf("Failed to reconcile new nodes for nodepool %q", kind.Existing.Nodepool)
				tracker.Diagnostics.Push(err)
				// since there currently is no mechanism for tracking partial changes out
				// of the terraform output, commit the changes and let manager work out the diff.
				//
				// fallthrough
			}
		}
	case *spec.Update_TerraformerAddLoadBalancerNodes_New_:
		current.ClusterInfo.NodePools = append(current.ClusterInfo.NodePools, kind.New.Nodepool)
		if err := lb.ReconcileNodePool(ctx, logger, kind.New.Nodepool.Name, loadbalancer.ReconcileModeReadWrite); err != nil {
			if errors.Is(err, loadbalancer.ErrReconcileAll) {
				err = lb.ReconcileAll(ctx, logger)
			}

			if err != nil {
				logger.Err(err).Msgf("Failed to reconcile new nodepool %q", kind.New.Nodepool.Name)
				tracker.Diagnostics.Push(err)
				// since there currently is no mechanism for tracking partial changes out
				// of the terraform output, commit the changes and let manager work out the diff.
				//
				// fallthrough
			}
		}
	default:
		logger.
			Warn().
			Msgf("Received add nodes to loadbalancers action, but with an invalid addition kind %T, ignoring", kind)
		return
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}
