package service

import (
	"fmt"
	"slices"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"google.golang.org/protobuf/proto"
)

// TODO: write tests.
// TODO: what if the key for the static nodes is changed that should trigger, or not that would
// be handled automatically on the run anyways.

type (
	// KubernetesDiffResult holds all of the changes between two different [spec.K8Scluster]
	KubernetesDiffResult struct {
		// Whether the kubernetes version changed.
		KubernetesVersion bool

		// Whether proxy settigns changed.
		Proxy bool

		// Diff in the Dynamic NodePools of the cluster.
		Dynamic NodePoolsDiffResult

		// Diff in the Static NodePools of the cluster.
		Static NodePoolsDiffResult

		// TODO: possibly include autoscaling diff here ?
		// TODO: endpointNodeDeleted function from managerV1.
		//
		// State of the Api endpoint for the kubernetes cluster.
		// If the kubernetes cluster has no api endpoint, but it
		// is in one of the loadbalancers attached to the cluster
		// both of the values will be empty.
		ApiEndpoint struct {
			// ID of the node which is used for the API server in the
			// current state. If there is no, the value will be empty.
			Current string

			// ID of the node which is used for the API server in the
			// desired state. If there is no, the value will be empty.
			Desired string
		}

		// TODO: should be done per nodepool below.
		// Changes made to the autoscaler nodepools.
		Autoscaler struct { /* TODO */
		}

		// Labels updated.
		Labels bool

		// Annotations updated.
		Annotations bool

		// Taints updated.
		Taints bool
	}

	ModifiedLoadBalancer struct {
		// DNS changed
		DNS bool

		// Changes related to roles.
		Roles struct {
			// Name of the roles added.
			Added []string

			// Name of the roles deleted.
			Deleted []string

			// Name of the roles to which TargetPools were added
			// with the name of the Pools that were added.
			TargetPoolsAdded TargetPoolsViewType

			// Name of the roles from which TargetPools were deleted
			// with the name of the Pools that were deleted.
			TargetPoolsDeleted TargetPoolsViewType
		}

		// Diff in the Dynamic NodePools of the cluster.
		Dynamic NodePoolsDiffResult

		// Diff in the Static NodePools of the cluster.
		Static NodePoolsDiffResult
	}

	// LoadBalancersDiffResult holds all of the changes between two different [spec.LoadBalancers] states
	LoadBalancersDiffResult struct {
		// IDs of the loadbalancer that were added.
		Added []string

		// IDs of the loadbalancers that were deleted.
		Deleted []string

		// LoadBalancers present in both views but have inner differences.
		Modified map[string]ModifiedLoadBalancer

		// State of the Api endpoint for the Loadbalancers.
		ApiEndpoint struct {
			// ID of the loadbalancer with the Api role in the current state.
			Current string

			// ID of the loadbalancers with the Api role in the desired state.
			Desired string

			// What action should be done based on the difference for the [Current] and [Desired].
			State spec.ApiEndpointChangeStateV2

			// Whether all of the nodepools in the kubernetes cluster's desired state
			// are deleted which the ApiEndpoint targets. This field is
			// mutually exclusive with all of the above [ApiEndpoint] fields.
			TargetPoolsDeleted bool
		}
	}

	// NodePoolsDiffResult holds all of the changes between two different [NodePoolViewType]
	NodePoolsDiffResult struct {
		// PartialDeleted holds which nodepools were partially deleted, meaning that
		// the nodepools is present in both [NodePoolsViewType] but some of the nodes
		// are missing.
		//
		// The data is stored as map[NodePool.Name][]node.Names
		// Each entry in the map contains the NodePool and the Nodes that are
		// present in the first[NodePoolsViewType] but not in the other.
		PartiallyDeleted NodePoolsViewType

		// Delete holds which nodepools were completely deleted, meaning that
		// they're present in one [NodePoolsViewType] but no the other.
		//
		// The data is stored as map[NodePool.Name][]node.Names
		// Each entry in the map contains all of the node Names that are
		// present in the first [NodePoolsViewType] but not in the other.
		Deleted NodePoolsViewType

		// Same as [NodePoolsDiffResult.partiallyDeleted] but with newly added nodes.
		PartiallyAdded NodePoolsViewType

		// Same as [NodePoolsDiffResult.deleted] but with newly added Nodepools and their nodes.
		Added NodePoolsViewType
	}

	// NodePoolsViewType is an unordered view into the nodepools and their nodes that are read from a [spec.K8Scluster].
	NodePoolsViewType = map[string][]string

	// TargetPoolsViewType is an unordered view into the diff for target pools that are from a [spec.Role].
	TargetPoolsViewType = map[string][]string
)

func (r *NodePoolsDiffResult) IsEmpty() bool {
	isEmpty := len(r.Deleted) == 0
	isEmpty = isEmpty && len(r.PartiallyDeleted) == 0
	isEmpty = isEmpty && len(r.Added) == 0
	isEmpty = isEmpty && len(r.PartiallyAdded) == 0
	return isEmpty
}

// NodePoolsDiff calculates difference between two [NodePoolsViewType] and returns the result as a [NodePoolsDiffResult].
func NodePoolsDiff(old, new NodePoolsViewType) NodePoolsDiffResult {
	result := NodePoolsDiffResult{
		PartiallyDeleted: NodePoolsViewType{},
		Deleted:          NodePoolsViewType{},
		PartiallyAdded:   NodePoolsViewType{},
		Added:            NodePoolsViewType{},
	}

	// 1. track nodepools that are completely absent in the new version.
	for nodepool, nodes := range old {
		if _, ok := new[nodepool]; !ok {
			result.Deleted[nodepool] = nodes
		}
	}
	// delete the nodepools from the old version
	for nodepool := range result.Deleted {
		delete(old, nodepool)
	}

	// 2. track nodepools that are new in the new version.
	for nodepool, nodes := range new {
		if _, ok := old[nodepool]; !ok {
			result.Added[nodepool] = nodes
		}
	}
	// delete the nodepools from the new version.
	for nodepool := range result.Added {
		delete(new, nodepool)
	}

	// Now, In both old,new [NodePoolsViewType] we only have nodepools that are present in both.

	// 3. track partially deleted node from nodepools present in both versions.
	for nodepool, oldNodes := range old {
		newNodes := new[nodepool]

		for _, oldNode := range oldNodes {
			found := slices.Contains(newNodes, oldNode)
			if !found {
				result.PartiallyDeleted[nodepool] = append(result.PartiallyDeleted[nodepool], oldNode)
			}
		}
	}
	// delete the partially deleted nodes from the old version.
	for nodepool, nodes := range result.PartiallyDeleted {
		old[nodepool] = slices.DeleteFunc(old[nodepool], func(node string) bool { return slices.Contains(nodes, node) })
	}

	// 4. track partially added nodes from nodepools present in both versions.
	for nodepool, newNodes := range new {
		oldNodes := old[nodepool]

		for _, newNode := range newNodes {
			found := slices.Contains(oldNodes, newNode)
			if !found {
				result.PartiallyAdded[nodepool] = append(result.PartiallyAdded[nodepool], newNode)
			}
		}
	}
	// delete the partially added nodes from the old version.
	for nodepool, nodes := range result.PartiallyAdded {
		new[nodepool] = slices.DeleteFunc(new[nodepool], func(node string) bool { return slices.Contains(nodes, node) })
	}

	// Now in the new,old [NodePoolsViewType] only the nodes that are common to both of them remain,
	// but we do not work with those in here thus we leave them as is.
	return result
}

// NodePoolsView returns a view into the individual dynamic and static NodePools of the
// passed in [spec.K8Scluster] struct and returns them as an [NodePoolsViewType].
func NodePoolsView(info *spec.ClusterInfoV2) (dynamic NodePoolsViewType, static NodePoolsViewType) {
	dynamic, static = make(NodePoolsViewType), make(NodePoolsViewType)

	for _, nodepool := range info.GetNodePools() {
		switch np := nodepool.GetType().(type) {
		case *spec.NodePool_DynamicNodePool:
			// the nodepool can have 0 nodes, for example when
			// the autocaler scales from 0.
			dynamic[nodepool.Name] = []string{}

			for _, node := range nodepool.Nodes {
				dynamic[nodepool.Name] = append(dynamic[nodepool.Name], node.Name)
			}
		case *spec.NodePool_StaticNodePool:
			// The nodepool could have 0 nodes.
			static[nodepool.Name] = []string{}

			for _, node := range nodepool.Nodes {
				static[nodepool.Name] = append(static[nodepool.Name], node.Name)
			}
		default:
			// Panic here as this is an error from the programmer if any new
			// nodepool types should be added in the future.
			panic(fmt.Sprintf("Unsupported NodePool Type: %T", np))
		}
	}

	return
}

func KubernetesDiff(old, new *spec.K8SclusterV2) KubernetesDiffResult {
	// TODO:

	var (
		odynamic, ostatic = NodePoolsView(old.GetClusterInfo())
		ndynamic, nstatic = NodePoolsView(new.GetClusterInfo())

		dynamicDiff = NodePoolsDiff(odynamic, ndynamic)
		staticDiff  = NodePoolsDiff(ostatic, nstatic)
	)

	return KubernetesDiffResult{
		KubernetesVersion: false,
		Proxy:             false,
		Dynamic:           dynamicDiff,
		Static:            staticDiff,
		ApiEndpoint: struct {
			Current string
			Desired string
		}{
			Current: "",
			Desired: "",
		},
		Autoscaler:  struct{}{},
		Labels:      false,
		Annotations: false,
		Taints:      false,
	}
}

func LoadBalancersDiff(k8sdiff *KubernetesDiffResult, old, new *spec.LoadBalancersV2) LoadBalancersDiffResult {
	result := LoadBalancersDiffResult{
		Modified: make(map[string]ModifiedLoadBalancer),
	}

	// 1. Find any added.
	for _, n := range new.Clusters {
		idx := clusters.IndexLoadbalancerByIdV2(n.ClusterInfo.Id(), old.Clusters)
		if idx < 0 {
			result.Added = append(result.Added, n.ClusterInfo.Id())
			continue
		}
	}

	// 2. Find any deleted/modified.
	for _, old := range old.Clusters {
		idx := clusters.IndexLoadbalancerByIdV2(old.ClusterInfo.Id(), new.Clusters)
		if idx < 0 {
			result.Deleted = append(result.Deleted, old.ClusterInfo.Id())
			continue
		}

		// 2.1 Find any difference between clusters that exist in both.
		new := new.Clusters[idx]

		var (
			// Changes in Roles
			rolesAdded         []string
			rolesDeleted       []string
			targetPoolsAdded   = make(TargetPoolsViewType)
			targetPoolsDeleted = make(TargetPoolsViewType)
		)

		for _, o := range old.Roles {
			found := slices.ContainsFunc(new.Roles, func(r *spec.RoleV2) bool { return o.Name == r.Name })
			if !found {
				rolesDeleted = append(rolesDeleted, o.Name)
			}
		}

		for _, n := range new.Roles {
			found := slices.ContainsFunc(old.Roles, func(r *spec.RoleV2) bool { return n.Name == r.Name })
			if !found {
				rolesAdded = append(rolesAdded, n.Name)
			}
		}

		for _, o := range old.Roles {
			var newRole *spec.RoleV2

			for _, n := range new.Roles {
				if n.Name == o.Name {
					newRole = n
					break
				}
			}

			if newRole == nil {
				continue
			}

			// TargetPools deleted.
			for _, old := range o.TargetPools {
				found := slices.Contains(newRole.TargetPools, old)
				if !found {
					targetPoolsDeleted[newRole.Name] = append(targetPoolsDeleted[newRole.Name], old)
				}
			}

			// TargetPools added
			for _, n := range newRole.TargetPools {
				found := slices.Contains(o.TargetPools, n)
				if !found {
					targetPoolsAdded[newRole.Name] = append(targetPoolsAdded[newRole.Name], n)
				}
			}
		}

		// Changes in DNS
		var dnsChanged bool
		switch {
		case old.Dns == nil && new.Dns != nil:
			dnsChanged = true
		case old.Dns != nil && new.Dns == nil:
			dnsChanged = true
		case old.Dns != nil && new.Dns != nil:
			if old.Dns.Provider.SpecName == new.Dns.Provider.SpecName {
				if proto.Equal(old.Dns, new.Dns) {
					break
				}
			}
			dnsChanged = true
		}

		// Changes in NodePools.
		oldDynamic, oldStatic := NodePoolsView(old.ClusterInfo)
		newDynamic, newStatic := NodePoolsView(new.ClusterInfo)

		dynDiff := NodePoolsDiff(oldDynamic, newDynamic)
		sttDiff := NodePoolsDiff(oldStatic, newStatic)

		modified := len(rolesAdded) > 0 || len(rolesDeleted) > 0
		modified = modified || len(targetPoolsAdded) > 0 || len(targetPoolsDeleted) > 0
		modified = modified || dnsChanged
		modified = modified || (!dynDiff.IsEmpty() || !sttDiff.IsEmpty())
		if modified {
			entry := struct {
				DNS   bool
				Roles struct {
					Added              []string
					Deleted            []string
					TargetPoolsAdded   TargetPoolsViewType
					TargetPoolsDeleted TargetPoolsViewType
				}
				Dynamic NodePoolsDiffResult
				Static  NodePoolsDiffResult
			}{}

			entry.DNS = dnsChanged

			entry.Roles.Added = rolesAdded
			entry.Roles.Deleted = rolesDeleted
			entry.Roles.TargetPoolsAdded = targetPoolsAdded
			entry.Roles.TargetPoolsDeleted = targetPoolsDeleted

			if !dynDiff.IsEmpty() {
				entry.Dynamic = dynDiff
			}

			if !sttDiff.IsEmpty() {
				entry.Static = dynDiff
			}

			result.Modified[old.ClusterInfo.Id()] = entry
		}
	}

	// 3. Determine API Endpoint changes.
	cid, did, change := clusters.DetermineLBApiEndpointChangeV2(old.Clusters, new.Clusters)
	result.ApiEndpoint.Current = cid
	result.ApiEndpoint.Desired = did
	result.ApiEndpoint.State = change
	result.ApiEndpoint.TargetPoolsDeleted = apiNodePoolsDeleted(k8sdiff, old)

	return result
}

func apiNodePoolsDeleted(k8sdiff *KubernetesDiffResult, old *spec.LoadBalancersV2) bool {
	search := make(map[string]struct{})

	for np := range k8sdiff.Dynamic.Deleted {
		search[np] = struct{}{}
	}

	for np := range k8sdiff.Static.Deleted {
		search[np] = struct{}{}
	}

	ep := clusters.FindAssignedLbApiEndpointV2(old.Clusters)
	for _, role := range ep.GetRoles() {
		if role.RoleType != spec.RoleTypeV2_ApiServer_V2 {
			continue
		}

		matched := 0
		for _, tp := range role.TargetPools {
			for np := range search {
				if name, _ := nodepools.MatchNameAndHashWithTemplate(tp, np); name == tp {
					matched += 1
					break
				}
			}
		}

		if matched == len(role.TargetPools) {
			return true
		}

		break
	}

	return false
}
