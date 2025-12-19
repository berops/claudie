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
	action, ok := tracker.Task.Do.(*spec.Task_Update)
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
	case *spec.Update_TfAddLoadBalancer:
		lb := delta.TfAddLoadBalancer.Handle
		addLoadBalancer(logger, projectName, processLimit, lb, tracker)
	case *spec.Update_DeleteLoadBalancerNodes_:
		action := DeleteLoadBalancerNodes{
			State:  state,
			Delete: delta.DeleteLoadBalancerNodes,
		}
		deleteLoadBalancerNodes(logger, projectName, processLimit, action, tracker)
	case *spec.Update_TfAddLoadBalancerNodes:
		action := AddLoadBalancerNodes{
			State: state,
			Add:   delta.TfAddLoadBalancerNodes,
		}
		addLoadBalancerNodes(logger, projectName, processLimit, action, tracker)
	case *spec.Update_DeleteLoadBalancerRoles_:
		action := DeleteLoadBalancerRoles{
			State:  state,
			Delete: delta.DeleteLoadBalancerRoles,
		}
		deleteLoadBalancerRoles(logger, projectName, processLimit, action, tracker)
	case *spec.Update_TfAddLoadBalancerRoles:
		action := AddLoadBalancerRoles{
			State: state,
			Add:   delta.TfAddLoadBalancerRoles,
		}
		addLoadBalancerRoles(logger, projectName, processLimit, action, tracker)
	case *spec.Update_TfReplaceDns:
		action := ReplaceDns{
			State:   state,
			Replace: delta.TfReplaceDns,
		}
		replaceDns(logger, projectName, processLimit, action, tracker)
	case *spec.Update_DeleteLoadBalancer_:
		id := delta.DeleteLoadBalancer.Handle
		destroyLoadBalancer(logger, projectName, id, state.LoadBalancers, processLimit, stores, tracker)
	case *spec.Update_DeleteK8SNodes_:
		action := DeleteKubernetesNodes{
			State:  state,
			Delete: delta.DeleteK8SNodes,
		}
		deleteKubernetesNodes(logger, projectName, processLimit, action, tracker)
	case *spec.Update_TfAddK8SNodes:
		action := AddKubernetesNodes{
			State: state,
			Add:   delta.TfAddK8SNodes,
		}
		addKubernetesNodes(logger, projectName, processLimit, action, tracker)
	default:
		logger.
			Warn().
			Msgf("Received update task with action %T, assuming the task was misscheduled, ignoring", action.Update.Delta)
		return
	}
}
