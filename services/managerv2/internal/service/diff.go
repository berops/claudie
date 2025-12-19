package service

import (
	"fmt"
	"slices"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"

	"google.golang.org/protobuf/proto"
)

// TODO: write tests.
// TODO: what if the key for the static nodes is changed that should trigger, or not that would
// be handled automatically on the run anyways.

type ProxyChange uint8

const (
	ProxyNoChange ProxyChange = iota
	ProxyOff
	ProxyOn
)

type (
	DiffResult struct {
		Kubernetes    KubernetesDiffResult
		LoadBalancers LoadBalancersDiffResult
	}

	ProxyDiffResult struct {
		// Whether the proxy is in use in current state.
		CurrentUsed bool

		// Whether the proxy is in use in the desired state.
		DesiredUsed bool

		// Whether there is any change from the current to the
		// desired state.
		Change ProxyChange
	}

	LabelsTaintsAnnotationsDiffResult struct {
		Deleted struct {
			LabelKeys       map[string][]string
			AnnotationsKeys map[string][]string
			TaintKeys       map[string][]*spec.Taint
		}
		Added struct {
			LabelKeys       map[string][]string
			AnnotationsKeys map[string][]string
			TaintKeys       map[string][]*spec.Taint
		}
	}

	// KubernetesDiffResult holds all of the changes between two different [spec.K8Scluster]
	KubernetesDiffResult struct {
		// Whether the kubernetes version changed.
		Version bool

		// Whether proxy settings changed.
		Proxy ProxyDiffResult

		// Diff in the Dynamic NodePools of the cluster.
		Dynamic NodePoolsDiffResult

		// Diff in the Static NodePools of the cluster.
		Static NodePoolsDiffResult

		// State of the Api endpoint for the kubernetes cluster.
		// If the kubernetes cluster has no api endpoint, but it
		// is in one of the loadbalancers attached to the cluster
		// both of the values will be empty.
		ApiEndpoint struct {
			// ID of the node which is used for the API server in the
			// current state. If there is no, the value will be empty.
			Current         string
			CurrentNodePool string

			// ID of the node which is used for the API server in the
			// desired state. If there is no, the value will be empty.
			Desired         string
			DesiredNodePool string
		}

		// TODO: should be done per nodepool below.
		// Changes made to the autoscaler nodepools.
		// TODO: possibly include autoscaling diff here ?
		Autoscaler struct { /* TODO */
		}

		// TODO move the diff from nodepools.go into here...
		LabelsTaintsAnnotations LabelsTaintsAnnotationsDiffResult
	}

	ModifiedLoadBalancer struct {
		// Index in the previous state.
		//
		// If changes to the current state are
		// made the index is invalidated.
		CurrentIdx int

		// Index in the new state.
		//
		// If changes to the new state are
		// made the index is invalidated.
		DesiredIdx int

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

	LoadBalancerIdentifier struct {
		// Id of the loadbalancer
		Id string

		// Index of the loadbalancer
		//
		// If the state from which the index
		// was obtained is modified, this
		// index is invalidated and must not
		// be used futher.
		Index int
	}

	// LoadBalancersDiffResult holds all of the changes between two different [spec.LoadBalancers] states
	LoadBalancersDiffResult struct {
		// Loadbalancers that were added in the desired state
		// but are not present in the current state.
		Added []LoadBalancerIdentifier

		// IDs of the loadbalancers that were deleted in the desired
		// state but are present in the current state.
		Deleted []LoadBalancerIdentifier

		// LoadBalancers present in both views but have inner differences.
		Modified map[string]ModifiedLoadBalancer

		// State of the Api endpoint for the Loadbalancers.
		ApiEndpoint struct {
			// ID of the loadbalancer with the Api role.
			Current string

			// ID of the loadbalancers to which the Api role should be transfered to.
			New string

			// What action should be done based on the difference for the [Current] and [Desired].
			State spec.ApiEndpointChangeState

			// Whether all of the nodepools in the kubernetes cluster's desired state
			// are deleted which the ApiEndpoint targets. This field is
			// mutually exclusive with all of the above [ApiEndpoint] fields.
			// TODO: handle, I don't think we need this information.
			// TODO: test this once kubernetes part is done.
			// TargetPoolsDeleted bool
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
func NodePoolsView(info *spec.ClusterInfo) (dynamic NodePoolsViewType, static NodePoolsViewType) {
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

func Diff(old, new *spec.Clusters) DiffResult {
	var result DiffResult

	result.Kubernetes = KubernetesDiff(old.K8S, new.K8S)
	result.LoadBalancers = LoadBalancersDiff(old.LoadBalancers, new.LoadBalancers)

	return result
}

func KubernetesDiff(old, new *spec.K8Scluster) KubernetesDiffResult {
	var (
		result KubernetesDiffResult

		odynamic, ostatic = NodePoolsView(old.GetClusterInfo())
		ndynamic, nstatic = NodePoolsView(new.GetClusterInfo())

		dynamicDiff = NodePoolsDiff(odynamic, ndynamic)
		staticDiff  = NodePoolsDiff(ostatic, nstatic)
	)

	proxyDiff := ProxyDiffResult{
		CurrentUsed: UsesProxy(old),
		DesiredUsed: UsesProxy(new),
	}

	if proxyDiff.CurrentUsed && !proxyDiff.DesiredUsed {
		proxyDiff.Change = ProxyOff
	}

	if !proxyDiff.CurrentUsed && proxyDiff.DesiredUsed {
		proxyDiff.Change = ProxyOn
	}

	result.Proxy = proxyDiff
	result.Version = old.Kubernetes != new.Kubernetes
	result.Dynamic = dynamicDiff
	result.Static = staticDiff

	// Check if the api endpoint is in the k8s cluster.
api:
	for _, np := range old.ClusterInfo.NodePools {
		if np.IsControl {
			for _, node := range np.Nodes {
				if node.NodeType == spec.NodeType_apiEndpoint {
					result.ApiEndpoint.Current = node.Name
					result.ApiEndpoint.CurrentNodePool = np.Name

					// Assume here that the desired state keeps the
					// Api server, if this will not be the case, then
					// the below check if handle that.

					result.ApiEndpoint.Desired = node.Name
					result.ApiEndpoint.DesiredNodePool = np.Name

					break api
				}
			}
		}
	}

	// Check if Api endpoint is deleted, based on the nodepools diff.
	if result.ApiEndpoint.Current != "" {
		if del, ok := result.Dynamic.Deleted[result.ApiEndpoint.CurrentNodePool]; ok {
			if slices.Contains(del, result.ApiEndpoint.Current) {
				np, ep := newAPIEndpointNodeCandidate(new.ClusterInfo.NodePools)
				result.ApiEndpoint.Desired = ep
				result.ApiEndpoint.DesiredNodePool = np
			}
		}

		if del, ok := result.Dynamic.PartiallyDeleted[result.ApiEndpoint.CurrentNodePool]; ok {
			if slices.Contains(del, result.ApiEndpoint.Current) {
				np, ep := newAPIEndpointNodeCandidate(new.ClusterInfo.NodePools)
				result.ApiEndpoint.Desired = ep
				result.ApiEndpoint.DesiredNodePool = np
			}
		}

		if del, ok := result.Static.Deleted[result.ApiEndpoint.CurrentNodePool]; ok {
			if slices.Contains(del, result.ApiEndpoint.Current) {
				np, ep := newAPIEndpointNodeCandidate(new.ClusterInfo.NodePools)
				result.ApiEndpoint.Desired = ep
				result.ApiEndpoint.DesiredNodePool = np
			}
		}

		if del, ok := result.Static.PartiallyDeleted[result.ApiEndpoint.CurrentNodePool]; ok {
			if slices.Contains(del, result.ApiEndpoint.Current) {
				np, ep := newAPIEndpointNodeCandidate(new.ClusterInfo.NodePools)
				result.ApiEndpoint.Desired = ep
				result.ApiEndpoint.DesiredNodePool = np
			}
		}
	}

	// labels,taints,annotaions diff.
	for _, c := range old.ClusterInfo.NodePools {
		for _, n := range new.ClusterInfo.NodePools {
			// Only perform the diff on NodePools in both
			// states. Is Old is not in New than it will
			// be deleted, and if New is not in Old than
			// it will be added with the correct ones.
			if c.Name != n.Name {
				continue
			}
			diff := result.LabelsTaintsAnnotations

			// deleted
			for k := range c.Labels {
				if _, ok := n.Labels[k]; !ok {
					diff.Deleted.LabelKeys[n.Name] = append(diff.Deleted.LabelKeys[n.Name], k)
				}
			}

			for k := range c.Annotations {
				if _, ok := n.Annotations[k]; !ok {
					diff.Deleted.AnnotationsKeys[n.Name] = append(diff.Deleted.AnnotationsKeys[n.Name], k)
				}
			}

			for _, t := range c.Taints {
				ok := slices.ContainsFunc(n.Taints, func(other *spec.Taint) bool {
					return other.Key == t.Key && other.Value == t.Value && other.Effect == t.Effect
				})
				if !ok {
					diff.Deleted.TaintKeys[n.Name] = append(diff.Deleted.TaintKeys[n.Name], &spec.Taint{
						Key:    t.Key,
						Value:  t.Value,
						Effect: t.Effect,
					})
				}
			}

			// added
			for k := range n.Labels {
				if _, ok := c.Labels[k]; !ok {
					diff.Added.LabelKeys[c.Name] = append(diff.Added.LabelKeys[c.Name], k)
				}
			}

			for k := range n.Annotations {
				if _, ok := c.Annotations[k]; !ok {
					diff.Added.AnnotationsKeys[c.Name] = append(diff.Added.AnnotationsKeys[c.Name], k)
				}
			}

			for _, t := range n.Taints {
				ok := slices.ContainsFunc(c.Taints, func(other *spec.Taint) bool {
					return other.Key == t.Key && other.Value == t.Value && other.Effect == t.Effect
				})
				if !ok {
					diff.Added.TaintKeys[c.Name] = append(diff.Added.TaintKeys[c.Name], &spec.Taint{
						Key:    t.Key,
						Value:  t.Value,
						Effect: t.Effect,
					})
				}
			}

			break
		}
	}

	return result
}

func LoadBalancersDiff(old, new *spec.LoadBalancers) LoadBalancersDiffResult {
	result := LoadBalancersDiffResult{
		Modified: make(map[string]ModifiedLoadBalancer),
	}

	// 1. Find any added.
	for i, n := range new.Clusters {
		idx := clusters.IndexLoadbalancerById(n.ClusterInfo.Id(), old.Clusters)
		if idx < 0 {
			result.Added = append(result.Added, LoadBalancerIdentifier{
				Id:    n.ClusterInfo.Id(),
				Index: i,
			})
			continue
		}
	}

	// 2. Find any deleted/modified.
	for oldIdx, old := range old.Clusters {
		idx := clusters.IndexLoadbalancerById(old.ClusterInfo.Id(), new.Clusters)
		if idx < 0 {
			result.Deleted = append(result.Deleted, LoadBalancerIdentifier{
				Id:    old.ClusterInfo.Id(),
				Index: oldIdx,
			})
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
			found := slices.ContainsFunc(new.Roles, func(r *spec.Role) bool { return o.Name == r.Name })
			if !found {
				rolesDeleted = append(rolesDeleted, o.Name)
			}
		}

		for _, n := range new.Roles {
			found := slices.ContainsFunc(old.Roles, func(r *spec.Role) bool { return n.Name == r.Name })
			if !found {
				rolesAdded = append(rolesAdded, n.Name)
			}
		}

		for _, o := range old.Roles {
			var newRole *spec.Role

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
				CurrentIdx int
				DesiredIdx int
				DNS        bool
				Roles      struct {
					Added              []string
					Deleted            []string
					TargetPoolsAdded   TargetPoolsViewType
					TargetPoolsDeleted TargetPoolsViewType
				}
				Dynamic NodePoolsDiffResult
				Static  NodePoolsDiffResult
			}{}

			entry.CurrentIdx = oldIdx
			entry.DesiredIdx = idx
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
	cid, did, change := determineLBApiEndpointChange(old.Clusters, new.Clusters)
	result.ApiEndpoint.Current = cid
	result.ApiEndpoint.New = did
	result.ApiEndpoint.State = change
	// TODO: remove me.
	// result.ApiEndpoint.TargetPoolsDeleted = apiNodePoolsDeleted(k8sdiff, old)

	return result
}

// func apiNodePoolsDeleted(k8sdiff *KubernetesDiffResult, old *spec.LoadBalancers) bool {
// 	search := make(map[string]struct{})

// 	for np := range k8sdiff.Dynamic.Deleted {
// 		search[np] = struct{}{}
// 	}

// 	for np := range k8sdiff.Static.Deleted {
// 		search[np] = struct{}{}
// 	}

// 	ep := clusters.FindAssignedLbApiEndpoint(old.Clusters)
// 	for _, role := range ep.GetRoles() {
// 		if role.RoleType != spec.RoleType_ApiServer {
// 			continue
// 		}

// 		matched := 0
// 		for _, tp := range role.TargetPools {
// 			for np := range search {
// 				if name, _ := nodepools.MatchNameAndHashWithTemplate(tp, np); name == tp {
// 					matched += 1
// 					break
// 				}
// 			}
// 		}

// 		if matched == len(role.TargetPools) {
// 			return true
// 		}

// 		break
// 	}

// 	return false
// }

func determineLBApiEndpointChange(
	currentLbs,
	desiredLbs []*spec.LBcluster,
) (string, string, spec.ApiEndpointChangeState) {
	var (
		none    string
		first   *spec.LBcluster
		desired = make(map[string]*spec.LBcluster)
	)

	for _, lb := range desiredLbs {
		if lb.HasApiRole() {
			desired[lb.ClusterInfo.Id()] = lb
			if first == nil {
				first = lb
			}
		}
	}

	if current := clusters.FindAssignedLbApiEndpoint(currentLbs); current != nil {
		if len(desired) == 0 && current.Dns == nil { // current state has no dns, but lb was deleted.
			return none, none, spec.ApiEndpointChangeState_NoChange
		}

		if len(desired) == 0 {
			return current.ClusterInfo.Id(), none, spec.ApiEndpointChangeState_DetachingLoadBalancer
		}
		if current.Dns == nil { // current state has no dns but there is at least one cluster in desired state.
			return none, first.ClusterInfo.Id(), spec.ApiEndpointChangeState_AttachingLoadBalancer
		}
		if desired, ok := desired[current.ClusterInfo.Id()]; ok {
			if current.Dns.Endpoint != desired.Dns.Endpoint {
				return current.ClusterInfo.Id(), first.ClusterInfo.Id(), spec.ApiEndpointChangeState_EndpointRenamed
			}
			return none, none, spec.ApiEndpointChangeState_NoChange
		}
		return current.ClusterInfo.Id(), first.ClusterInfo.Id(), spec.ApiEndpointChangeState_MoveEndpoint
	} else {
		if len(desired) == 0 {
			return none, none, spec.ApiEndpointChangeState_NoChange
		}

		return none, first.ClusterInfo.Id(), spec.ApiEndpointChangeState_AttachingLoadBalancer
	}
}

func newAPIEndpointNodeCandidate(desired []*spec.NodePool) (string, string) {
	for _, np := range desired {
		if np.IsControl {
			// There should always be a control node, this is an invariant
			// by checked at the validation level.
			return np.Name, np.Nodes[0].Name
		}
	}

	// This should never happen as the validation forbids not having
	// any control plane nodes.
	panic("no suitable api endpoint replacement candidate found, malformed state.")
}
