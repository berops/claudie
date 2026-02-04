package spec

import (
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"
)

// TODO: tests

// Consumes the [TaskResult_Clear] for the task.
func (te *Task) ConsumeClearResult(result *TaskResult_Clear) error {
	var k8s **K8Scluster
	var lbs *[]*LBcluster

	switch task := te.Do.(type) {
	case *Task_Create:
		k8s = &task.Create.K8S
		lbs = &task.Create.LoadBalancers
	case *Task_Delete:
		k8s = &task.Delete.K8S
		lbs = &task.Delete.LoadBalancers
	case *Task_Update:
		k8s = &task.Update.State.K8S
		lbs = &task.Update.State.LoadBalancers
	default:
		return nil
	}

	// first acknowledge any clearing of loadbalancers.
	lbFilter := func(lb *LBcluster) bool {
		return slices.Contains(result.Clear.LoadBalancersIDs, lb.GetClusterInfo().Id())
	}
	*lbs = slices.DeleteFunc(*lbs, lbFilter)

	if result.Clear.K8S != nil && *result.Clear.K8S {
		// if the clear removes also the kuberentes cluster
		// perform an additional check that the rest of the
		// infrastructure is deleted, i.e there are no more
		// loadbalancers.
		if len(*lbs) > 0 {
			// This will result in a partial consumption of the clear
			// message while ignoring the other part.
			return fmt.Errorf("refuse to clear kubernetes cluster while loadbalancers for it still exists")
		}

		*k8s = nil
		*lbs = nil

		// fallthrough
	}

	return nil
}

// Consumes the [TaskResult_Update] for the task.
func (te *Task) ConsumeUpdateResult(result *TaskResult_Update) error {
	var k8s **K8Scluster
	var lbs *[]*LBcluster

	switch task := te.Do.(type) {
	case *Task_Create:
		k8s = &task.Create.K8S
		lbs = &task.Create.LoadBalancers
	case *Task_Delete:
		k8s = &task.Delete.K8S
		lbs = &task.Delete.LoadBalancers
	case *Task_Update:
		k8s = &task.Update.State.K8S
		lbs = &task.Update.State.LoadBalancers
	default:
		return nil
	}

	id := (*k8s).ClusterInfo.Id()
	name := (*k8s).ClusterInfo.Name

	// Consume delete updates, if any.
	if update := te.GetUpdate(); update != nil {
		switch delta := update.Delta.(type) {
		case *Update_KDeleteNodes:
			idx := slices.IndexFunc((*k8s).ClusterInfo.NodePools, func(n *NodePool) bool {
				return n.Name == delta.KDeleteNodes.Nodepool
			})
			if idx < 0 {
				// Under normal circumstances this should never happen, this signals
				// either malformed/corrupted data and/or mistake in the schedule of
				// tasks. Thus rather return an error than continiung with the merge.
				return fmt.Errorf("can't update cluster %q received update result with invalid deleted nodepool %q", id, delta.KDeleteNodes.Nodepool)
			}

			affected := (*k8s).ClusterInfo.NodePools[idx]
			unreachable := delta.KDeleteNodes.Unreachable

			if delta.KDeleteNodes.WithNodePool {
				update.Delta = &Update_DeletedK8SNodes_{
					DeletedK8SNodes: &Update_DeletedK8SNodes{
						Unreachable: unreachable,
						Kind: &Update_DeletedK8SNodes_Whole{
							Whole: &Update_DeletedK8SNodes_WholeNodePool{
								// Below, with the replacement of the kuberentes
								// cluster it should no longer reference this nodepool
								// and this should be the only owner of it afterwards.
								Nodepool: affected,
							},
						},
					},
				}
			} else {
				d := &Update_DeletedK8SNodes_Partial_{
					Partial: &Update_DeletedK8SNodes_Partial{
						Nodepool:       delta.KDeleteNodes.Nodepool,
						Nodes:          []*Node{},
						StaticNodeKeys: map[string]string{},
					},
				}

				// Below, with the replacement of the kubernetes
				// cluster it should no longer reference these nodes
				// and this should be the only owner of them afterwards.
				for _, n := range affected.Nodes {
					if slices.Contains(delta.KDeleteNodes.Nodes, n.Name) {
						d.Partial.Nodes = append(d.Partial.Nodes, n)
					}
				}

				if stt := affected.GetStaticNodePool(); stt != nil {
					for _, n := range d.Partial.Nodes {
						key := n.Public
						d.Partial.StaticNodeKeys[key] = stt.NodeKeys[key]
					}
				}

				update.Delta = &Update_DeletedK8SNodes_{
					DeletedK8SNodes: &Update_DeletedK8SNodes{
						Unreachable: unreachable,
						Kind:        d,
					},
				}
			}
		case *Update_TfDeleteLoadBalancerNodes:
			handle := delta.TfDeleteLoadBalancerNodes.Handle
			lbi := slices.IndexFunc(*lbs, func(lb *LBcluster) bool {
				return lb.ClusterInfo.Id() == handle
			})
			if lbi < 0 {
				// Under normal circumstances this should never happen, this signals
				// either malformed/corrupted data and/or mistake in the schedule of
				// tasks. Thus rather return an error than continiung with the merge.
				return fmt.Errorf("can't update loadbalancer %q received update result with invalid loadbalancer id", id)
			}

			lb := (*lbs)[lbi]

			npi := slices.IndexFunc(lb.ClusterInfo.NodePools, func(n *NodePool) bool {
				return n.Name == delta.TfDeleteLoadBalancerNodes.Nodepool
			})
			if npi < 0 {
				// Under normal circumstances this should never happen, this signals
				// either malformed/corrupted data and/or mistake in the schedule of
				// tasks. Thus rather return an error than continiung with the merge.
				return fmt.Errorf("can't update loadbalancer %q received update result with invalid deleted nodepool %q", id, delta.TfDeleteLoadBalancerNodes.Nodepool)
			}

			affected := lb.ClusterInfo.NodePools[npi]
			unreachable := delta.TfDeleteLoadBalancerNodes.Unreachable

			if delta.TfDeleteLoadBalancerNodes.WithNodePool {
				update.Delta = &Update_DeletedLoadBalancerNodes_{
					DeletedLoadBalancerNodes: &Update_DeletedLoadBalancerNodes{
						Unreachable: unreachable,
						Handle:      handle,
						Kind: &Update_DeletedLoadBalancerNodes_Whole{
							Whole: &Update_DeletedLoadBalancerNodes_WholeNodePool{
								// Below, with the replacement of the loadbalancer
								// cluster it should no longer reference this nodepool
								// and this should be the only owner of it afterwards.
								Nodepool: affected,
							},
						},
					},
				}
			} else {
				d := &Update_DeletedLoadBalancerNodes_Partial_{
					Partial: &Update_DeletedLoadBalancerNodes_Partial{
						Nodepool:       delta.TfDeleteLoadBalancerNodes.Nodepool,
						Nodes:          []*Node{},
						StaticNodeKeys: map[string]string{},
					},
				}

				// Below, with the replacement of the loadbalancer cluster
				// it should no longer reference these nodes and this should
				// be the only owner of them afterwards.
				for _, n := range affected.Nodes {
					if slices.Contains(delta.TfDeleteLoadBalancerNodes.Nodes, n.Name) {
						d.Partial.Nodes = append(d.Partial.Nodes, n)
					}
				}

				if stt := affected.GetStaticNodePool(); stt != nil {
					for _, n := range d.Partial.Nodes {
						key := n.Public
						d.Partial.StaticNodeKeys[key] = stt.NodeKeys[key]
					}
				}

				update.Delta = &Update_DeletedLoadBalancerNodes_{
					DeletedLoadBalancerNodes: &Update_DeletedLoadBalancerNodes{
						Unreachable: unreachable,
						Handle:      handle,
						Kind:        d,
					},
				}
			}
		}
	}

	if k := result.Update.K8S; k != nil {
		if gotName := k.GetClusterInfo().Id(); gotName != id {
			// Under normal circumstances this should never happen, this signals either
			// malformed/corrupted data and/or mistake in the scheduling of tasks.
			// Thus return an error rather than continuing with the merge.
			return fmt.Errorf("can't update cluster %q with received cluster %q", id, gotName)
		}
		(*k8s) = k
		result.Update.K8S = nil
	}

	toUpdate := LoadBalancers{
		Clusters: result.Update.LoadBalancers.Clusters,
	}
	result.Update.LoadBalancers.Clusters = nil

	toUpdate.Clusters = slices.DeleteFunc(toUpdate.Clusters, func(lb *LBcluster) bool {
		return lb.TargetedK8S != name
	})

	// update existing ones.
	for i := range *lbs {
		lb := (*lbs)[i].ClusterInfo.Id()
		for j := range toUpdate.Clusters {
			if update := toUpdate.Clusters[j].ClusterInfo.Id(); lb == update {
				(*lbs)[i] = toUpdate.Clusters[j]
				toUpdate.Clusters = slices.Delete(toUpdate.Clusters, j, j+1)
				break
			}
		}
	}

	// add new ones.
	*lbs = append(*lbs, toUpdate.Clusters...)

	// Consume replace updates, if any.
	if update := te.GetUpdate(); update != nil {
		switch delta := update.Delta.(type) {
		case *Update_TfMoveNodePoolFromAutoscaled:
			nodepool := delta.TfMoveNodePoolFromAutoscaled.Nodepool
			var config *AutoscalerConf

			for _, np := range update.State.K8S.ClusterInfo.NodePools {
				if np.Name == nodepool {
					if dyn := np.GetDynamicNodePool(); dyn != nil {
						config = dyn.AutoscalerConfig
					}
				}
			}

			update.Delta = &Update_MovedNodePoolFromAutoscaled_{
				MovedNodePoolFromAutoscaled: &Update_MovedNodePoolFromAutoscaled{
					Nodepool: nodepool,
					Config:   config,
				},
			}
		case *Update_TfMoveNodePoolToAutoscaled:
			nodepool := delta.TfMoveNodePoolToAutoscaled.Nodepool
			update.Delta = &Update_MovedNodePoolToAutoscaled_{
				MovedNodePoolToAutoscaled: &Update_MovedNodePoolToAutoscaled{
					Nodepool: nodepool,
				},
			}
		case *Update_AnsReplaceProxy:
			update.Delta = &Update_ReplacedProxy{
				ReplacedProxy: &Update_ReplacedProxySettings{},
			}
		case *Update_AnsReplaceTargetPools:
			consumed := &Update_ReplacedTargetPools{
				Handle: delta.AnsReplaceTargetPools.Handle,
				Roles:  map[string]*Update_ReplacedTargetPools_TargetPools{},
			}

			for k, v := range delta.AnsReplaceTargetPools.Roles {
				consumed.Roles[k] = &Update_ReplacedTargetPools_TargetPools{
					Pools: v.Pools,
				}
			}

			update.Delta = &Update_ReplacedTargetPools_{
				ReplacedTargetPools: consumed,
			}
		case *Update_KpatchNodes:
			update.Delta = &Update_PatchedNodes_{
				PatchedNodes: &Update_PatchedNodes{},
			}
		case *Update_TfAddK8SNodes:
			consumed := &Update_AddedK8SNodes{
				NewNodePool: false,
				Nodepool:    "",
				Nodes:       []string{},
			}

			switch kind := delta.TfAddK8SNodes.Kind.(type) {
			case *Update_TerraformerAddK8SNodes_Existing_:
				consumed.NewNodePool = false
				consumed.Nodepool = kind.Existing.Nodepool
				for _, n := range kind.Existing.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			case *Update_TerraformerAddK8SNodes_New_:
				consumed.NewNodePool = true
				consumed.Nodepool = kind.New.Nodepool.Name
				for _, n := range kind.New.Nodepool.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			}

			update.Delta = &Update_AddedK8SNodes_{
				AddedK8SNodes: consumed,
			}
		case *Update_TfAddLoadBalancer:
			update.Delta = &Update_AddedLoadBalancer_{
				AddedLoadBalancer: &Update_AddedLoadBalancer{
					Handle: delta.TfAddLoadBalancer.Handle.ClusterInfo.Id(),
				},
			}
		case *Update_TfAddLoadBalancerNodes:
			consumed := &Update_AddedLoadBalancerNodes{
				Handle:      delta.TfAddLoadBalancerNodes.Handle,
				NewNodePool: false,
				NodePool:    "",
				Nodes:       []string{},
			}

			switch kind := delta.TfAddLoadBalancerNodes.Kind.(type) {
			case *Update_TerraformerAddLoadBalancerNodes_Existing_:
				consumed.NewNodePool = false
				consumed.NodePool = kind.Existing.Nodepool
				for _, n := range kind.Existing.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			case *Update_TerraformerAddLoadBalancerNodes_New_:
				consumed.NewNodePool = true
				consumed.NodePool = kind.New.Nodepool.Name
				for _, n := range kind.New.Nodepool.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			}

			update.Delta = &Update_AddedLoadBalancerNodes_{
				AddedLoadBalancerNodes: consumed,
			}
		case *Update_TfAddLoadBalancerRoles:
			roles := make([]string, 0, len(delta.TfAddLoadBalancerRoles.Roles))
			for _, r := range delta.TfAddLoadBalancerRoles.Roles {
				roles = append(roles, r.Name)
			}
			update.Delta = &Update_AddedLoadBalancerRoles_{
				AddedLoadBalancerRoles: &Update_AddedLoadBalancerRoles{
					Handle: delta.TfAddLoadBalancerRoles.Handle,
					Roles:  roles,
				},
			}
		case *Update_TfReplaceDns:
			update.Delta = &Update_ReplacedDns_{
				ReplacedDns: &Update_ReplacedDns{
					Handle:         delta.TfReplaceDns.Handle,
					OldApiEndpoint: delta.TfReplaceDns.OldApiEndpoint,
				},
			}
		default:
			// other messages are non-consumable, do nothing.
		}
	}

	return nil
}

// Returns mutable references to the underlying [Clusters] state stored
// within the [Task]. Any changes made to the returned [Clusters] will
// be reflected in the individual [Task] state.
//
// Each [Task] is spawned with a valid [Clusters] state, thus the function
// always returns fully valid data which was scheduled for the task.
//
// While this allows to directly mutate the returned [Clusters] it will not
// allow Clearing, i.e setting to nil. There is a more unsafe function available
// ,when needed, [ReplaceClusters] which replaces the state of the task otherwise
// consider gradually upgrading the task via the [Task.ConsumeClearResult] or
// [Task.ConsumeUpdateResult]
func (te *Task) MutableClusters() (*Clusters, error) {
	state := Clusters{
		LoadBalancers: &LoadBalancers{},
	}

	switch task := te.Do.(type) {
	case *Task_Create:
		state.K8S = task.Create.K8S
		state.LoadBalancers.Clusters = task.Create.LoadBalancers
	case *Task_Delete:
		state.K8S = task.Delete.K8S
		state.LoadBalancers.Clusters = task.Delete.LoadBalancers
	case *Task_Update:
		state.K8S = task.Update.State.K8S
		state.LoadBalancers.Clusters = task.Update.State.LoadBalancers
	default:
		return nil, fmt.Errorf("unknown task %T", task)
	}

	return &state, nil
}

// Replaces the [K8Scluster] and its [LBcluster]s with the ones provided in
// the passed in [Clusters] object. The values are consumed, and the ones
// inside the passed in [Clusters] are set to nil. After calling this function
// the passed in references should no longer be used and the [Task] should be
// the only owner of them.
func (te *Task) ReplaceClusters(c *Clusters) {
	switch task := te.Do.(type) {
	case *Task_Create:
		task.Create.K8S = c.K8S
		task.Create.LoadBalancers = c.LoadBalancers.Clusters
	case *Task_Delete:
		task.Delete.K8S = c.K8S
		task.Delete.LoadBalancers = c.LoadBalancers.Clusters
	case *Task_Update:
		task.Update.State.K8S = c.K8S
		task.Update.State.LoadBalancers = c.LoadBalancers.Clusters
	default:
		// Do nothing.
	}

	c.K8S = nil
	c.LoadBalancers = nil
}

type InFlightUpdateState struct {
	r     *TaskResult
	state *TaskResult_UpdateState
}

func (s *InFlightUpdateState) Kubernetes(c *K8Scluster) *InFlightUpdateState {
	if c != nil {
		s.state.K8S = proto.Clone(c).(*K8Scluster)
	}
	return s
}

func (s *InFlightUpdateState) Loadbalancers(lbs ...*LBcluster) *InFlightUpdateState {
	if len(lbs) > 0 {
		s.state.LoadBalancers = new(LoadBalancers)
		for _, lb := range lbs {
			if lb != nil {
				lb := proto.Clone(lb).(*LBcluster)
				s.state.LoadBalancers.Clusters = append(s.state.LoadBalancers.Clusters, lb)
			}
		}
	}
	return s
}

func (s *InFlightUpdateState) Commit() {
	switch prev := s.r.Result.(type) {
	case *TaskResult_Update:
		old := prev.Update
		desired := s.state

		if desired.K8S != nil {
			old.K8S = desired.K8S
			desired.K8S = nil
		}

		// update existing ones.
		for i := range old.LoadBalancers.Clusters {
			o := old.LoadBalancers.Clusters[i].ClusterInfo.Id()
			for j := range desired.LoadBalancers.Clusters {
				if n := desired.LoadBalancers.Clusters[j].ClusterInfo.Id(); n == o {
					old.LoadBalancers.Clusters[i] = desired.LoadBalancers.Clusters[j]
					desired.LoadBalancers.Clusters = slices.Delete(desired.LoadBalancers.Clusters, j, j+1)
					break
				}
			}
		}

		// add new ones
		old.LoadBalancers.Clusters = append(old.LoadBalancers.Clusters, desired.LoadBalancers.Clusters...)
		s.state = nil
	default:
		s.r.Result = &TaskResult_Update{
			Update: s.state,
		}
		s.state = nil
	}
}

func (r *TaskResult) Update() *InFlightUpdateState {
	return &InFlightUpdateState{
		r: r,
		state: &TaskResult_UpdateState{
			LoadBalancers: &LoadBalancers{},
		},
	}
}

type InFlightClearState struct {
	r     *TaskResult
	state *TaskResult_ClearState
}

func (s *InFlightClearState) Commit() {
	s.r.Result = &TaskResult_Clear{
		Clear: s.state,
	}
	s.state = nil
}

func (s *InFlightClearState) Kubernetes() *InFlightClearState {
	ok := true
	s.state.K8S = &ok
	return s
}

func (s *InFlightClearState) LoadBalancers(lbs ...string) *InFlightClearState {
	if len(lbs) > 0 {
		s.state.LoadBalancersIDs = []string{}
		s.state.LoadBalancersIDs = append(s.state.LoadBalancersIDs, lbs...)
	}
	return s
}

func (r *TaskResult) Clear() *InFlightClearState {
	return &InFlightClearState{
		r:     r,
		state: new(TaskResult_ClearState),
	}
}

func (r *TaskResult) IsNone() bool   { return r.GetNone() != nil }
func (r *TaskResult) IsUpdate() bool { return r.GetUpdate() != nil }
func (r *TaskResult) IsClear() bool  { return r.GetClear() != nil }
