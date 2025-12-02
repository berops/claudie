package service

import (
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

func reconcileInfrastructure(
	logger zerolog.Logger,
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	action, ok := tracker.Task.Do.(*spec.TaskV2_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to update infrastructure, assuming the task was misscheduled, ignoring", tracker.Task.Do)
		return
	}

	state := action.Update.State
	if state == nil || state.K8S == nil {
		logger.Warn().Msg("Update task validation failed, required state of the kuberentes cluster to be present, ignoring")
		return
	}

	switch delta := action.Update.Delta.(type) {
	case *spec.UpdateV2_DeleteLoadBalancer_:
		id := delta.DeleteLoadBalancer.Handle
		destroyLoadBalancer(logger, projectName, id, state.LoadBalancers, processLimit, stores, tracker)
	case *spec.UpdateV2_TfAddLoadBalancer:
		lb := delta.TfAddLoadBalancer.Handle
		reconcileLoadBalancer(logger, projectName, processLimit, lb, tracker)
	case *spec.UpdateV2_TfReconcileLoadBalancer:
		lb := delta.TfReconcileLoadBalancer.Handle
		reconcileLoadBalancer(logger, projectName, processLimit, lb, tracker)
	case *spec.UpdateV2_TfReplaceDns:
		dns := delta.TfReplaceDns
		replaceDns(logger, projectName, processLimit, state, dns, tracker)
	default:
		logger.
			Warn().
			Msgf("Received update task with action %T, assuming the task was misscheduled, ignoring", action.Update.Delta)
		return
	}
}
