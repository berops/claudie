package service

import (
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/internal/worker/service/internal/nodes"
	"github.com/rs/zerolog"
)

func DeleteNodes(logger zerolog.Logger, tracker Tracker) {
	action, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task for deleting nodes that is of type %T, which is not an update, ignoring", tracker.Task.Do)
		return
	}

	d, ok := action.Update.Delta.(*spec.Update_KDeleteNodes)
	if !ok {
		logger.
			Warn().
			Msgf("Received task for deleting nodes that is of type %T, which is not for deleeting k8s nodes, ignoring", action.Update.Delta)
		return
	}

	k8s := action.Update.State.K8S
	np := nodepools.FindByName(d.KDeleteNodes.Nodepool, k8s.ClusterInfo.NodePools)
	if np == nil {
		logger.
			Warn().
			Msgf("Received valid task for deleting nodes, but the nodepools %q from which nodes are "+
				"to be deleted is missing from the provided state, ignoring", d.KDeleteNodes.Nodepool)
		return
	}

	var (
		master []string
		worker []string
	)

	if np.IsControl {
		master = append(master, d.KDeleteNodes.Nodes...)
		logger.Info().Msgf("Deleting %v control nodes", len(master))
	} else {
		worker = append(worker, d.KDeleteNodes.Nodes...)
		logger.Info().Msgf("Deleting %v worker nodes", len(worker))
	}

	if len(master) == 0 && len(worker) == 0 {
		return
	}

	deleter, err := nodes.NewDeleter(master, worker, k8s)
	if err != nil {
		logger.Err(err).Msg("Failed to prepare node deletion")
		tracker.Diagnostics.Push(err)
		return
	}

	if err := deleter.DeleteNodes(logger); err != nil {
		logger.Err(err).Msg("Failed to delete nodes")
		tracker.Diagnostics.Push(err)
		return
	}

	if d.KDeleteNodes.WithNodePool {
		k8s.ClusterInfo.NodePools = nodepools.DeleteByName(k8s.ClusterInfo.NodePools, d.KDeleteNodes.Nodepool)
	} else {
		nodepools.DeleteNodes(np, d.KDeleteNodes.Nodes)
	}

	update := tracker.Result.Update()
	update.Kubernetes(k8s)
	update.Commit()

	logger.Info().Msg("Nodes successfully deleted")
}
