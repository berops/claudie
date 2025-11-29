package utils

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
)

// Reads the state of the clusters for the received task. On unknown state
// no state is returned and `false` indicating failure.
func StateFromTask(task *spec.TaskV2) (*spec.K8SclusterV2, []*spec.LBclusterV2, bool) {
	switch task := task.GetDo().(type) {
	case *spec.TaskV2_Create:
		return task.Create.K8S, task.Create.LoadBalancers, true
	case *spec.TaskV2_Update:
		return task.Update.State.K8S, task.Update.State.LoadBalancers, true
	default:
		return nil, nil, false
	}
}

// If the task is of [spec.Update_ReconciledLoadBalancer] or [spec.Update_AddedLoadBalancer]
// instead of keeping all of the loadbalancers in lbs slices, only the loadbalancer for
// which the reconciliation is called is kept in the lbs slice.
func DefaultToSingleLoadBalancerIfPossible(task *spec.TaskV2, lbs []*spec.LBclusterV2) []*spec.LBclusterV2 {
	if len(lbs) == 0 {
		return lbs
	}

	u, ok := task.Do.(*spec.TaskV2_Update)
	if !ok {
		return lbs
	}

	switch delta := u.Update.Delta.(type) {
	case *spec.UpdateV2_AddedLoadBalancer_:
		if i := clusters.IndexLoadbalancerByIdV2(delta.AddedLoadBalancer.Handle, lbs); i >= 0 {
			lb := lbs[i]
			clear(lbs)
			lbs = lbs[:0]
			return append(lbs, lb)
		}
	case *spec.UpdateV2_ReconciledLoadBalancer_:
		if i := clusters.IndexLoadbalancerByIdV2(delta.ReconciledLoadBalancer.Handle, lbs); i >= 0 {
			lb := lbs[i]
			clear(lbs)
			lbs = lbs[:0]
			return append(lbs, lb)
		}
	}

	return lbs
}
