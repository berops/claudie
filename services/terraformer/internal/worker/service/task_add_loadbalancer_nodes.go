package service

import (
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
	// Currently there is no special mechanism for adding the
	// nodes of the loadbalancer as the whole cluster shares a
	// single state file, thus simply just add the new nodes to
	// the state and reconcile the cluster.
	idx := clusters.IndexLoadbalancerById(action.Add.Handle, action.State.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't add nodes for loadbalancer %q that is missing from the received state", action.Add.Handle)
		return
	}

	current := action.State.LoadBalancers[idx]

	switch kind := action.Add.Kind.(type) {
	case *spec.Update_TerraformerAddLoadBalancerNodes_Existing_:
		np := nodepools.FindByName(kind.Existing.Nodepool, current.ClusterInfo.NodePools)
		if np == nil {
			logger.
				Warn().
				Msgf(
					"Can't add nodes to nodepool %q of loadbalancer %q as the nodepool is missing form the received state",
					kind.Existing.Nodepool,
					action.Add.Handle,
				)
			return
		}

		if np.GetStaticNodePool() != nil {
			// Static nodes are not, and should be not, added through the
			// terraformer stage, thus here we can only focus on considering that
			// the nodes to be added here are dynamic nodes.
			logger.
				Warn().
				Msgf(
					"Can't work with static nodes from nodepool %q within loadbalancer %q, as their infrastructure cannot be managed by claudie, ignoring",
					np.Name,
					action.Add.Handle,
				)
			return
		}

		nodepools.DynamicAddNodes(np, kind.Existing.Nodes)
	case *spec.Update_TerraformerAddLoadBalancerNodes_New_:
		current.ClusterInfo.NodePools = append(current.ClusterInfo.NodePools, kind.New.Nodepool)
	default:
		logger.
			Warn().
			Msgf("Received add nodes to loadbalancers action, but with an invalid addition kind %T, ignoring", kind)
		return
	}

	lb := loadbalancer.LBcluster{
		ProjectName:       projectName,
		Cluster:           current,
		SpawnProcessLimit: processLimit,
	}

	buildLogger := logger.With().Str("cluster", lb.Id()).Logger()
	if err := BuildLoadbalancers(buildLogger, lb); err != nil {
		buildLogger.Err(err).Msg("Failed to reconcile cluster after nodes addition")
		tracker.Diagnostics.Push(err)
		// Contrary to the deletion process, during the addition if any partial changes
		// take effect we have to report them back, however since there is currently
		// no mechanism for tracking partial changes out of the terraform output
		// commit the whole changes, and let manager work out the diff.
		//
		// fallthrough
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb.Cluster)
	update.Commit()
}
