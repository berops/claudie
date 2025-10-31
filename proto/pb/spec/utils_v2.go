package spec

import (
	"fmt"
	"slices"

	"github.com/gogo/protobuf/proto"
)

// Id returns the ID of the cluster.
func (c *ClusterInfoV2) Id() string {
	if c == nil {
		return ""
	}
	return fmt.Sprintf("%s-%s", c.Name, c.Hash)
}

// DynamicNodePools returns slice of dynamic node pools.
func (c *ClusterInfoV2) DynamicNodePools() []*DynamicNodePool {
	if c == nil {
		return nil
	}

	nps := make([]*DynamicNodePool, 0, len(c.NodePools))
	for _, np := range c.NodePools {
		if n := np.GetDynamicNodePool(); n != nil {
			nps = append(nps, n)
		}
	}

	return nps
}

// AnyAutoscaledNodePools returns true, if cluster has at least one nodepool with autoscaler config.
func (c *K8SclusterV2) AnyAutoscaledNodePools() bool {
	if c == nil {
		return false
	}

	for _, np := range c.ClusterInfo.NodePools {
		if n := np.GetDynamicNodePool(); n != nil {
			if n.AutoscalerConfig != nil {
				return true
			}
		}
	}

	return false
}

func (c *K8SclusterV2) NodeCount() int {
	var out int

	if c == nil {
		return out
	}

	for _, np := range c.ClusterInfo.NodePools {
		switch i := np.Type.(type) {
		case *NodePool_DynamicNodePool:
			out += int(i.DynamicNodePool.Count)
		case *NodePool_StaticNodePool:
			out += len(i.StaticNodePool.NodeKeys)
		}
	}

	return out
}

func (c *LBclusterV2) NodeCount() int {
	var out int

	if c == nil {
		return out
	}

	for _, np := range c.ClusterInfo.NodePools {
		switch i := np.Type.(type) {
		case *NodePool_DynamicNodePool:
			out += int(i.DynamicNodePool.Count)
		case *NodePool_StaticNodePool:
			// Lbs are only dynamic.
		}
	}

	return out
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

func (s *InFlightUpdateState) TakeKubernetesCluster(c *K8SclusterV2) *InFlightUpdateState {
	if c != nil {
		s.state.K8S = proto.Clone(c).(*K8SclusterV2)
	}
	return s
}

func (s *InFlightUpdateState) TakeLoadBalancers(lbs ...*LBclusterV2) *InFlightUpdateState {
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

func (s *InFlightUpdateState) Replace() {
	s.r.Result = &TaskResult_Update{
		Update: s.state,
	}
	s.state = nil
}

func (r *TaskResult) KeepAsIs() {}

func (r *TaskResult) ToUpdate() *InFlightUpdateState {
	return &InFlightUpdateState{
		r:     r,
		state: new(TaskResult_UpdateState),
	}
}

type InFlightClearState struct {
	r     *TaskResult
	state *TaskResult_ClearState
}

func (s *InFlightClearState) Replace() {
	s.r.Result = &TaskResult_Clear{
		Clear: s.state,
	}
	s.state = nil
}

func (s *InFlightClearState) TakeKuberentesCluster(ok bool) *InFlightClearState {
	if ok {
		s.state.K8S = &ok
	}
	return s
}

func (s *InFlightClearState) TakeLoadBalancers(lbs ...string) *InFlightClearState {
	if len(lbs) > 0 {
		s.state.LoadBalancersIDs = []string{}
		for _, lb := range lbs {
			s.state.LoadBalancersIDs = append(s.state.LoadBalancersIDs, lb)
		}
	}
	return s
}

func (r *TaskResult) ToClear() *InFlightClearState {
	return &InFlightClearState{
		r:     r,
		state: new(TaskResult_ClearState),
	}
}

func (r *TaskResult) IsNone() bool   { return r.GetNone() != nil }
func (r *TaskResult) IsUpdate() bool { return r.GetUpdate() != nil }
func (r *TaskResult) IsClear() bool  { return r.GetClear() != nil }
