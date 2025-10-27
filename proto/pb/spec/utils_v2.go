package spec

import (
	"fmt"
	"slices"
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
