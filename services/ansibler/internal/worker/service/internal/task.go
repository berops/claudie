package utils

import "github.com/berops/claudie/proto/pb/spec"

// Reads the state of the clusters for the supported tasks.
// TODO: find a better name.
func ClustersFromTask(task *spec.TaskV2) (*spec.K8SclusterV2, []*spec.LBclusterV2, bool) {
	switch task := task.GetDo().(type) {
	case *spec.TaskV2_Create:
		return task.Create.K8S, task.Create.LoadBalancers, true
	case *spec.TaskV2_Update:
		return task.Update.State.K8S, task.Update.State.LoadBalancers, true
	default:
		return nil, nil, false
	}
}
