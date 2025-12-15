package spec

import (
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"
)

// Consumes the [TaskResult_Clear] for the task.
func (te *TaskV2) ConsumeClearResult(result *TaskResult_Clear) {
	var k8s **K8SclusterV2
	var lbs *[]*LBclusterV2

	switch task := te.Do.(type) {
	case *TaskV2_Create:
		k8s = &task.Create.K8S
		lbs = &task.Create.LoadBalancers
	case *TaskV2_Delete:
		k8s = &task.Delete.K8S
		lbs = &task.Delete.LoadBalancers
	case *TaskV2_Update:
		k8s = &task.Update.State.K8S
		lbs = &task.Update.State.LoadBalancers
	default:
		return
	}

	if result.Clear.K8S != nil && *result.Clear.K8S {
		*k8s = nil
		*lbs = nil
		return
	}

	lbFilter := func(lb *LBclusterV2) bool {
		return slices.Contains(result.Clear.LoadBalancersIDs, lb.GetClusterInfo().Id())
	}
	*lbs = slices.DeleteFunc(*lbs, lbFilter)
}

// Consumes the [TaskResult_Update] for the task.
func (te *TaskV2) ConsumeUpdateResult(result *TaskResult_Update) error {
	var k8s **K8SclusterV2
	var lbs *[]*LBclusterV2

	switch task := te.Do.(type) {
	case *TaskV2_Create:
		k8s = &task.Create.K8S
		lbs = &task.Create.LoadBalancers
	case *TaskV2_Delete:
		k8s = &task.Delete.K8S
		lbs = &task.Delete.LoadBalancers
	case *TaskV2_Update:
		k8s = &task.Update.State.K8S
		lbs = &task.Update.State.LoadBalancers
	default:
		return nil
	}

	id := (*k8s).GetClusterInfo().Id()
	name := (*k8s).GetClusterInfo().GetName()
	if k := result.Update.K8S; k != nil {
		if gotName := k.GetClusterInfo().Id(); gotName != id {
			// Under normal circumstances this should never happen, this signals either
			// malformed/corrupted data and/or mistake in the scheduling of tasks.
			// Thus return an error rather than continuing with the merge.
			return fmt.Errorf("Can't update cluster %q with received cluster %q", id, gotName)
		}
		(*k8s) = k
		result.Update.K8S = nil
	}

	var toUpdate LoadBalancersV2
	for _, lb := range result.Update.LoadBalancers.Clusters {
		toUpdate.Clusters = append(toUpdate.Clusters, lb)
	}
	result.Update.LoadBalancers.Clusters = nil

	toUpdate.Clusters = slices.DeleteFunc(toUpdate.Clusters, func(lb *LBclusterV2) bool {
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

	if update := te.GetUpdate(); update != nil {
		switch delta := update.Delta.(type) {
		case *UpdateV2_AnsReplaceProxy:
			update.Delta = &UpdateV2_ReplacedProxy{
				ReplacedProxy: &UpdateV2_ReplacedProxySettings{},
			}
		case *UpdateV2_AnsReplaceTargetPools:
			consumed := &UpdateV2_ReplacedTargetPools{
				Handle: delta.AnsReplaceTargetPools.Handle,
				Roles:  map[string]*UpdateV2_ReplacedTargetPools_TargetPools{},
			}

			for k, v := range delta.AnsReplaceTargetPools.Roles {
				consumed.Roles[k] = &UpdateV2_ReplacedTargetPools_TargetPools{
					Pools: v.Pools,
				}
			}

			update.Delta = &UpdateV2_ReplacedTargetPools_{
				ReplacedTargetPools: consumed,
			}
		case *UpdateV2_KpatchNodes:
			update.Delta = &UpdateV2_PatchedNodes_{
				PatchedNodes: &UpdateV2_PatchedNodes{},
			}
		case *UpdateV2_TfAddK8SNodes:
			consumed := &UpdateV2_AddedK8SNodes{
				NewNodePool: false,
				Nodepool:    "",
				Nodes:       []string{},
			}

			switch kind := delta.TfAddK8SNodes.Kind.(type) {
			case *UpdateV2_TerraformerAddK8SNodes_Existing_:
				consumed.NewNodePool = false
				consumed.Nodepool = kind.Existing.Nodepool
				for _, n := range kind.Existing.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			case *UpdateV2_TerraformerAddK8SNodes_New_:
				consumed.NewNodePool = true
				consumed.Nodepool = kind.New.Nodepool.Name
				for _, n := range kind.New.Nodepool.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			}

			update.Delta = &UpdateV2_AddedK8SNodes_{
				AddedK8SNodes: consumed,
			}
		case *UpdateV2_TfAddLoadBalancer:
			update.Delta = &UpdateV2_AddedLoadBalancer_{
				AddedLoadBalancer: &UpdateV2_AddedLoadBalancer{
					Handle: delta.TfAddLoadBalancer.Handle.ClusterInfo.Id(),
				},
			}
		case *UpdateV2_TfAddLoadBalancerNodes:
			consumed := &UpdateV2_AddedLoadBalancerNodes{
				Handle:      delta.TfAddLoadBalancerNodes.Handle,
				NewNodePool: false,
				NodePool:    "",
				Nodes:       []string{},
			}

			switch kind := delta.TfAddLoadBalancerNodes.Kind.(type) {
			case *UpdateV2_TerraformerAddLoadBalancerNodes_Existing_:
				consumed.NewNodePool = false
				consumed.NodePool = kind.Existing.Nodepool
				for _, n := range kind.Existing.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			case *UpdateV2_TerraformerAddLoadBalancerNodes_New_:
				consumed.NewNodePool = true
				consumed.NodePool = kind.New.Nodepool.Name
				for _, n := range kind.New.Nodepool.Nodes {
					consumed.Nodes = append(consumed.Nodes, n.Name)
				}
			}

			update.Delta = &UpdateV2_AddedLoadBalancerNodes_{
				AddedLoadBalancerNodes: consumed,
			}
		case *UpdateV2_TfAddLoadBalancerRoles:
			roles := make([]string, 0, len(delta.TfAddLoadBalancerRoles.Roles))
			for _, r := range delta.TfAddLoadBalancerRoles.Roles {
				roles = append(roles, r.Name)
			}
			update.Delta = &UpdateV2_AddedLoadBalancerRoles_{
				AddedLoadBalancerRoles: &UpdateV2_AddedLoadBalancerRoles{
					Handle: delta.TfAddLoadBalancerRoles.Handle,
					Roles:  roles,
				},
			}
		case *UpdateV2_TfReplaceDns:
			update.Delta = &UpdateV2_ReplacedDns_{
				ReplacedDns: &UpdateV2_ReplacedDns{
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
// allow Clearing, i.e setting to nil. For this consider using [Task.ConsumeClearResult]
// or [Task.ConsumeUpdateResult]
func (te *TaskV2) MutableClusters() (*ClustersV2, error) {
	state := ClustersV2{
		LoadBalancers: &LoadBalancersV2{},
	}

	switch task := te.Do.(type) {
	case *TaskV2_Create:
		state.K8S = task.Create.K8S
		state.LoadBalancers.Clusters = task.Create.LoadBalancers
	case *TaskV2_Delete:
		state.K8S = task.Delete.K8S
		state.LoadBalancers.Clusters = task.Delete.LoadBalancers
	case *TaskV2_Update:
		state.K8S = task.Update.State.K8S
		state.LoadBalancers.Clusters = task.Update.State.LoadBalancers
	default:
		return nil, fmt.Errorf("unknown task %T", task)
	}

	return &state, nil
}

// Id returns the ID of the cluster.
func (c *ClusterInfoV2) Id() string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%s-%s", c.Name, c.Hash)
}

// HasApiRole checks whether the LB has a role with port 6443.
func (c *LBclusterV2) HasApiRole() bool {
	if c == nil {
		return false
	}

	for _, role := range c.Roles {
		if role.RoleType == RoleTypeV2_ApiServer_V2 {
			return true
		}
	}

	return false
}

// IsApiEndpoint  checks whether the LB is selected as the API endpoint.
func (c *LBclusterV2) IsApiEndpoint() bool {
	if c == nil {
		return false
	}
	return c.HasApiRole() && c.UsedApiEndpoint
}

// TODO: remove any unsued functions after the refactor.
// MergeTargetPools takes the target pools from the other role
// and adds them to this role, ignoring duplicates.
func (r *RoleV2) MergeTargetPools(o *Role) {
	for _, o := range o.TargetPools {
		found := slices.Contains(r.TargetPools, o)
		if !found {
			// append missing target pool.
			r.TargetPools = append(r.TargetPools, o)
		}
	}
}

type InFlightUpdateState struct {
	r     *TaskResult
	state *TaskResult_UpdateState
}

func (s *InFlightUpdateState) Kubernetes(c *K8SclusterV2) *InFlightUpdateState {
	if c != nil {
		s.state.K8S = proto.Clone(c).(*K8SclusterV2)
	}
	return s
}

func (s *InFlightUpdateState) Loadbalancers(lbs ...*LBclusterV2) *InFlightUpdateState {
	if len(lbs) > 0 {
		s.state.LoadBalancers = new(LoadBalancersV2)
		for _, lb := range lbs {
			if lb != nil {
				lb := proto.Clone(lb).(*LBclusterV2)
				s.state.LoadBalancers.Clusters = append(s.state.LoadBalancers.Clusters, lb)
			}
		}
	}
	return s
}

// TODO: test me.
func (s *InFlightUpdateState) Commit() {
	switch prev := s.r.Result.(type) {
	case *TaskResult_Update:
		old := prev.Update
		new := s.state

		if new.K8S != nil {
			old.K8S = new.K8S
			new.K8S = nil
		}

		// update existing ones.
		for i := range old.LoadBalancers.Clusters {
			o := old.LoadBalancers.Clusters[i].ClusterInfo.Id()
			for j := range new.LoadBalancers.Clusters {
				if n := new.LoadBalancers.Clusters[j].ClusterInfo.Id(); n == o {
					old.LoadBalancers.Clusters[i] = new.LoadBalancers.Clusters[j]
					new.LoadBalancers.Clusters = slices.Delete(new.LoadBalancers.Clusters, j, j+1)
					break
				}
			}
		}

		// add new ones
		old.LoadBalancers.Clusters = append(old.LoadBalancers.Clusters, new.LoadBalancers.Clusters...)
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
			LoadBalancers: &LoadBalancersV2{},
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
		for _, lb := range lbs {
			s.state.LoadBalancersIDs = append(s.state.LoadBalancersIDs, lb)
		}
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
