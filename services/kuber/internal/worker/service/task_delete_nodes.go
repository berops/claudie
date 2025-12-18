package service

import (
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/internal/worker/service/internal/nodes"
	"github.com/rs/zerolog"
)

func DeleteNodes(logger zerolog.Logger, tracker Tracker) {
	action, ok := tracker.Task.Do.(*spec.TaskV2_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task for deleting nodes that is of type %T, which is not an update, ignoring", tracker.Task.Do)
		return
	}

	d, ok := action.Update.Delta.(*spec.UpdateV2_DeleteK8SNodes_)
	if !ok {
		logger.
			Warn().
			Msgf("Received task for deleting nodes that is of type %T, which is not for deleeting k8s nodes, ignoring", action.Update.Delta)
		return
	}

	np := nodepools.FindByName(d.DeleteK8SNodes.Nodepool, action.Update.State.K8S.ClusterInfo.NodePools)
	if np == nil {
		logger.
			Warn().
			Msgf("Received valid task for deleting nodes, but the nodepools %q from which nodes are "+
				"to be deleted is missing from the provided state, ignoring", d.DeleteK8SNodes.Nodepool)
		return
	}

	var (
		master        []string
		worker        []string
		keepNodepools = make(map[string]struct{})
	)

	if np.IsControl {
		master = append(master, d.DeleteK8SNodes.Nodes...)
	} else {
		worker = append(worker, d.DeleteK8SNodes.Nodes...)
	}

	if !d.DeleteK8SNodes.WithNodePool {
		keepNodepools[d.DeleteK8SNodes.Nodepool] = struct{}{}
	}

	if len(master) > 0 {
		logger.Info().Msgf("Deleting %v control nodes", len(master))
	}

	if len(worker) > 0 {
		logger.Info().Msgf("Deleting %v worker nodes", len(worker))
	}

	if len(master) == 0 && len(worker) == 0 {
		return
	}

	deleter := nodes.NewDeleter(master, worker, action.Update.State.K8S, keepNodepools)
	if err := deleter.DeleteNodes(); err != nil {
		logger.Err(err).Msg("Failed to delete nodes")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Nodes successfully deleted")
}
