package service

import (
	"fmt"
	"slices"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"

	"google.golang.org/protobuf/proto"
)

type ProxyChange uint8

const (
	ProxyNoChange ProxyChange = iota
	ProxyOff
	ProxyOn
	ProxyRefresh
)

type Labels = map[string]string
type Annotations = map[string]string
type Taints = []*spec.Taint

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
			Labels      map[string]Labels
			Annotations map[string]Annotations
			Taints      map[string]Taints
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

		// Nodepools to switch to autoscaling nodepools.
		ChangedToAutoscaled PendingAutoscaledNodePoolTransitions

		// Nodepools to switch to fixed nodepools.
		ChangedToFixed PendingFixedNodePoolTransitions

		// Dynamic nodes that are present in both the current and
		// desired state and are marked with [spec.NodeStatus_MarkedForDeletion]
		//
		// Reason why its included in the diff, is mostly that its a
		// pending diff to be handled at some point in the future.
		PendingDynamicDeletions PendingDeletionsViewType

		// Static nodes that are present in both the current and
		// desired state and are marked with [spec.NodeStatus_MarkedForDeletion]
		//
		// Reason why its included in the diff, is mostly that its a
		// pending diff to be handled at some point in the future.
		PendingStaticDeletions PendingDeletionsViewType

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

		LabelsTaintsAnnotations LabelsTaintsAnnotationsDiffResult

		// RollingUpdates are nodepools present in both states but
		// having different templates commit hash.
		RollingUpdates PendingRollingUpdates
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

			// Name of the roles for which internal settings have changed.
			// Internal Settings are:
			//  - Role.Settings
			//  - TargetPort
			InternalSettingsChanged []string

			// Name of the roles for which external settings have changed.
			// External settings are:
			// 	- Port
			// 	- Protocol
			ExternalSettingsChanged []string
		}

		// Diff in the Dynamic NodePools of the cluster.
		Dynamic NodePoolsDiffResult

		// Diff in the Static NodePools of the cluster.
		Static NodePoolsDiffResult

		// Dynamic nodes that are present in both the current and
		// desired state and are marked with [spec.NodeStatus_MarkedForDeletion]
		//
		// Reason why its included in the diff, is mostly that its a
		// pending diff to be handled at some point in the future.
		PendingDynamicDeletions PendingDeletionsViewType

		// Static nodes that are present in both the current and
		// desired state and are marked with [spec.NodeStatus_MarkedForDeletion]
		//
		// Reason why its included in the diff, is mostly that its a
		// pending diff to be handled at some point in the future.
		PendingStaticDeletions PendingDeletionsViewType

		// Nodepools that have their templates changed.
		RollingUpdate PendingRollingUpdates
	}

	LoadBalancerIdentifier struct {
		// Id of the loadbalancer
		Id string

		// Index of the loadbalancer
		//
		// If the state from which the index
		// was obtained is modified, this
		// index is invalidated and must not
		// be used further.
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

			// ID of the loadbalancers to which the Api role should be transferred to.
			New string

			// What action should be done based on the difference for the [Current] and [Desired].
			State spec.ApiEndpointChangeState
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

	// NodePoolsViewType is an unordered view into the nodepools and their
	// nodes that are read from a [spec.K8Scluster] or a [spec.LBcluster].
	NodePoolsViewType = map[string][]string

	// PendingNodeDeletions is an unordered view into the nodepools and their
	// nodes that are marked with [spec.NodeStatus_MarkedForDeletion]
	PendingDeletionsViewType = map[string][]string

	// PendingNodePoolTransitions is an unordered view into the nodepools
	// which has moved to autoscaled type from normal.
	PendingAutoscaledNodePoolTransitions = map[string]*spec.AutoscalerConf

	// PendingFixedNodePoolTransitions is an unordered view into the nodepools
	// which has move from autoscaled to fixed type.
	PendingFixedNodePoolTransitions = map[string]struct{}

	// PendingRollingUpdates is an unordered view into nodepools which
	// are present in both the current and desired state but have different
	// templates versions, meaning that a rolling update is required for the
	// infrastructure.
	PendingRollingUpdates map[string]*spec.TemplateRepository

	// TargetPoolsViewType is an unordered view into the diff for target pools
	// that are from a [spec.Role].
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
func NodePoolsDiff(old, desired NodePoolsViewType) NodePoolsDiffResult {
	result := NodePoolsDiffResult{
		PartiallyDeleted: NodePoolsViewType{},
		Deleted:          NodePoolsViewType{},
		PartiallyAdded:   NodePoolsViewType{},
		Added:            NodePoolsViewType{},
	}

	// 1. track nodepools that are completely absent in the new version.
	for nodepool, nodes := range old {
		if _, ok := desired[nodepool]; !ok {
			result.Deleted[nodepool] = nodes
		}
	}
	// delete the nodepools from the old version
	for nodepool := range result.Deleted {
		delete(old, nodepool)
	}

	// 2. track nodepools that are new in the new version.
	for nodepool, nodes := range desired {
		if _, ok := old[nodepool]; !ok {
			result.Added[nodepool] = nodes
		}
	}
	// delete the nodepools from the new version.
	for nodepool := range result.Added {
		delete(desired, nodepool)
	}

	// Now, In both old,new [NodePoolsViewType] we only have nodepools that are present in both.

	// 3. track partially deleted node from nodepools present in both versions.
	for nodepool, oldNodes := range old {
		newNodes := desired[nodepool]

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
	for nodepool, newNodes := range desired {
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
		desired[nodepool] = slices.DeleteFunc(desired[nodepool], func(node string) bool { return slices.Contains(nodes, node) })
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

func Diff(old, desired *spec.Clusters) DiffResult {
	var result DiffResult

	result.Kubernetes = KubernetesDiff(old.K8S, desired.K8S)
	result.LoadBalancers = LoadBalancersDiff(old.LoadBalancers, desired.LoadBalancers)

	return result
}

func KubernetesDiff(old, desired *spec.K8Scluster) KubernetesDiffResult {
	var (
		result KubernetesDiffResult

		odynamic, ostatic = NodePoolsView(old.GetClusterInfo())
		ndynamic, nstatic = NodePoolsView(desired.GetClusterInfo())

		dynamicDiff = NodePoolsDiff(odynamic, ndynamic)
		staticDiff  = NodePoolsDiff(ostatic, nstatic)
	)

	// Check for [spec.NodeStatus_MarkedForDeletion] nodes.
	// Check for nodepools that have moved from/to autoscaled type.
	//
	// Alot of nested for loops here but its not expected in real usage
	// to have a lot of nodepools in a cluster, along with a lot of nodes
	// especially since a nodepool has a limit of 255 nodes.
	//
	// Note: can be improved upon.
	result.PendingDynamicDeletions = make(PendingDeletionsViewType)
	result.PendingStaticDeletions = make(PendingDeletionsViewType)
	result.ChangedToAutoscaled = make(PendingAutoscaledNodePoolTransitions)
	result.ChangedToFixed = make(PendingFixedNodePoolTransitions)
	result.RollingUpdates = make(PendingRollingUpdates)
	for _, cnp := range old.ClusterInfo.NodePools {
		for _, dnp := range desired.ClusterInfo.NodePools {
			if cnp.Name != dnp.Name {
				continue
			}

			if cnp.GetDynamicNodePool() != nil && dnp.GetDynamicNodePool() != nil {
				cdyn := cnp.GetDynamicNodePool()
				ddyn := dnp.GetDynamicNodePool()

				if cdyn.Provider.Templates.CommitHash != ddyn.Provider.Templates.CommitHash {
					result.RollingUpdates[dnp.Name] = proto.Clone(ddyn.Provider.Templates).(*spec.TemplateRepository)
				}
				if cdyn.AutoscalerConfig == nil && ddyn.AutoscalerConfig != nil {
					result.ChangedToAutoscaled[dnp.Name] = proto.Clone(ddyn.AutoscalerConfig).(*spec.AutoscalerConf)
				}
				if cdyn.AutoscalerConfig != nil && ddyn.AutoscalerConfig == nil {
					result.ChangedToFixed[dnp.Name] = struct{}{}
				}
			}

			for _, cn := range cnp.Nodes {
				for _, dn := range dnp.Nodes {
					if cn.Name != dn.Name {
						continue
					}

					isPending := cn.Status == dn.Status
					isPending = isPending && cn.Status == spec.NodeStatus_MarkedForDeletion
					if isPending {
						if dnp.GetDynamicNodePool() != nil {
							result.PendingDynamicDeletions[dnp.Name] = append(result.PendingDynamicDeletions[dnp.Name], dn.Name)
						} else {
							result.PendingStaticDeletions[dnp.Name] = append(result.PendingStaticDeletions[dnp.Name], dn.Name)
						}
					}
				}
			}
		}
	}

	proxyDiff := ProxyDiffResult{
		CurrentUsed: UsesProxy(old),
		DesiredUsed: UsesProxy(desired),
	}

	if proxyDiff.CurrentUsed && !proxyDiff.DesiredUsed {
		proxyDiff.Change = ProxyOff
	}

	if !proxyDiff.CurrentUsed && proxyDiff.DesiredUsed {
		proxyDiff.Change = ProxyOn
	}

	if proxyDiff.CurrentUsed && proxyDiff.DesiredUsed {
		if old.InstallationProxy.NoProxy != desired.InstallationProxy.NoProxy {
			proxyDiff.Change = ProxyRefresh
		}
	}

	result.Proxy = proxyDiff
	result.Version = old.Kubernetes != desired.Kubernetes
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
					// the below check will handle that.

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
				np, ep := newAPIEndpointNodeCandidate(desired.ClusterInfo.NodePools)
				result.ApiEndpoint.Desired = ep
				result.ApiEndpoint.DesiredNodePool = np
			}
		}

		if del, ok := result.Dynamic.PartiallyDeleted[result.ApiEndpoint.CurrentNodePool]; ok {
			if slices.Contains(del, result.ApiEndpoint.Current) {
				np, ep := newAPIEndpointNodeCandidate(desired.ClusterInfo.NodePools)
				result.ApiEndpoint.Desired = ep
				result.ApiEndpoint.DesiredNodePool = np
			}
		}

		if del, ok := result.Static.Deleted[result.ApiEndpoint.CurrentNodePool]; ok {
			if slices.Contains(del, result.ApiEndpoint.Current) {
				np, ep := newAPIEndpointNodeCandidate(desired.ClusterInfo.NodePools)
				result.ApiEndpoint.Desired = ep
				result.ApiEndpoint.DesiredNodePool = np
			}
		}

		if del, ok := result.Static.PartiallyDeleted[result.ApiEndpoint.CurrentNodePool]; ok {
			if slices.Contains(del, result.ApiEndpoint.Current) {
				np, ep := newAPIEndpointNodeCandidate(desired.ClusterInfo.NodePools)
				result.ApiEndpoint.Desired = ep
				result.ApiEndpoint.DesiredNodePool = np
			}
		}
	}

	result.LabelsTaintsAnnotations.Deleted.AnnotationsKeys = make(map[string][]string)
	result.LabelsTaintsAnnotations.Deleted.LabelKeys = make(map[string][]string)
	result.LabelsTaintsAnnotations.Deleted.TaintKeys = make(map[string][]*spec.Taint)

	result.LabelsTaintsAnnotations.Added.Annotations = make(map[string]Annotations)
	result.LabelsTaintsAnnotations.Added.Labels = make(map[string]Labels)
	result.LabelsTaintsAnnotations.Added.Taints = make(map[string]Taints)

	// labels,taints,annotaions diff.
	for _, c := range old.ClusterInfo.NodePools {
		for _, n := range desired.ClusterInfo.NodePools {
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
			for k, nv := range n.Labels {
				if ov, ok := c.Labels[k]; !ok || nv != ov {
					if _, ok := diff.Added.Labels[c.Name]; !ok {
						diff.Added.Labels[c.Name] = make(Labels)
					}

					diff.Added.Labels[c.Name][k] = nv
				}
			}

			for k, nv := range n.Annotations {
				if ov, ok := c.Annotations[k]; !ok || nv != ov {
					if _, ok := diff.Added.Annotations[c.Name]; !ok {
						diff.Added.Annotations[c.Name] = make(Annotations)
					}

					diff.Added.Annotations[c.Name][k] = nv
				}
			}

			for _, t := range n.Taints {
				ok := slices.ContainsFunc(c.Taints, func(other *spec.Taint) bool {
					return other.Key == t.Key && other.Value == t.Value && other.Effect == t.Effect
				})
				if !ok {
					diff.Added.Taints[c.Name] = append(diff.Added.Taints[c.Name], &spec.Taint{
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

func LoadBalancersDiff(old, desired *spec.LoadBalancers) LoadBalancersDiffResult {
	result := LoadBalancersDiffResult{
		Modified: make(map[string]ModifiedLoadBalancer),
	}

	// 1. Find any added.
	for i, n := range desired.Clusters {
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
		idx := clusters.IndexLoadbalancerById(old.ClusterInfo.Id(), desired.Clusters)
		if idx < 0 {
			result.Deleted = append(result.Deleted, LoadBalancerIdentifier{
				Id:    old.ClusterInfo.Id(),
				Index: oldIdx,
			})
			continue
		}

		// 2.1 Find any difference between clusters that exist in both.
		desired := desired.Clusters[idx]

		var (
			// Changes in Roles
			internalSettingsChanged []string
			externalSettingsChanged []string
			rolesAdded              []string
			rolesDeleted            []string
			targetPoolsAdded        = make(TargetPoolsViewType)
			targetPoolsDeleted      = make(TargetPoolsViewType)
		)

		for _, o := range old.Roles {
			found := slices.ContainsFunc(desired.Roles, func(r *spec.Role) bool { return o.Name == r.Name })
			if !found {
				rolesDeleted = append(rolesDeleted, o.Name)
			}
		}

		for _, n := range desired.Roles {
			found := slices.ContainsFunc(old.Roles, func(r *spec.Role) bool { return n.Name == r.Name })
			if !found {
				rolesAdded = append(rolesAdded, n.Name)
			}
		}

		for _, o := range old.Roles {
			var newRole *spec.Role

			for _, n := range desired.Roles {
				if n.Name == o.Name {
					newRole = n
					break
				}
			}

			if newRole == nil {
				continue
			}

			hasInternalChanges := o.TargetPort != newRole.TargetPort
			hasInternalChanges = hasInternalChanges || (!proto.Equal(o.Settings, newRole.Settings))
			if hasInternalChanges {
				internalSettingsChanged = append(internalSettingsChanged, newRole.Name)
			}

			hasExternalChanges := o.Port != newRole.Port
			hasExternalChanges = hasExternalChanges || o.Protocol != newRole.Protocol
			if hasExternalChanges {
				externalSettingsChanged = append(externalSettingsChanged, newRole.Name)
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
		case old.Dns == nil && desired.Dns != nil:
			dnsChanged = true
		case old.Dns != nil && desired.Dns == nil:
			dnsChanged = true
		case old.Dns != nil && desired.Dns != nil:
			if old.Dns.Provider.SpecName == desired.Dns.Provider.SpecName {
				if proto.Equal(old.Dns, desired.Dns) {
					break
				}
			}
			dnsChanged = true
		}

		// Pending deletions.
		pendingDynamicDeletions := make(PendingDeletionsViewType)
		pendingStaticDeletions := make(PendingDeletionsViewType)
		rollingUpdates := make(PendingRollingUpdates)

		// Check for [spec.NodeStatus_MarkedForDeletion] nodes.
		//
		// Alot of nested for loops here but its not expected in real usage
		// to have a lot of nodepools in a cluster, along with a lot of nodes
		// especially since a nodepool has a limit of 255 nodes.
		//
		// Note: can be improved upon.
		for _, cnp := range old.ClusterInfo.NodePools {
			for _, dnp := range desired.ClusterInfo.NodePools {
				if cnp.Name != dnp.Name {
					continue
				}

				if cnp.GetDynamicNodePool() != nil && dnp.GetDynamicNodePool() != nil {
					cdyn := cnp.GetDynamicNodePool()
					ddyn := dnp.GetDynamicNodePool()

					if cdyn.Provider.Templates.CommitHash != ddyn.Provider.Templates.CommitHash {
						rollingUpdates[dnp.Name] = proto.Clone(ddyn.Provider.Templates).(*spec.TemplateRepository)
					}
				}

				for _, cn := range cnp.Nodes {
					for _, dn := range dnp.Nodes {
						if cn.Name != dn.Name {
							continue
						}

						isPending := cn.Status == dn.Status
						isPending = isPending && cn.Status == spec.NodeStatus_MarkedForDeletion
						if isPending {
							if dnp.GetDynamicNodePool() != nil {
								pendingDynamicDeletions[dnp.Name] = append(pendingDynamicDeletions[dnp.Name], dn.Name)
							} else {
								pendingStaticDeletions[dnp.Name] = append(pendingStaticDeletions[dnp.Name], dn.Name)
							}
						}
					}
				}
			}
		}

		// Changes in NodePools.
		oldDynamic, oldStatic := NodePoolsView(old.ClusterInfo)
		newDynamic, newStatic := NodePoolsView(desired.ClusterInfo)

		dynDiff := NodePoolsDiff(oldDynamic, newDynamic)
		sttDiff := NodePoolsDiff(oldStatic, newStatic)

		modified := len(rolesAdded) > 0 || len(rolesDeleted) > 0
		modified = modified || len(internalSettingsChanged) > 0 || len(externalSettingsChanged) > 0
		modified = modified || len(targetPoolsAdded) > 0 || len(targetPoolsDeleted) > 0
		modified = modified || dnsChanged
		modified = modified || len(pendingDynamicDeletions) > 0 || len(pendingStaticDeletions) > 0
		modified = modified || len(rollingUpdates) > 0
		modified = modified || (!dynDiff.IsEmpty() || !sttDiff.IsEmpty())
		if modified {
			entry := struct {
				CurrentIdx int
				DesiredIdx int
				DNS        bool
				Roles      struct {
					Added                   []string
					Deleted                 []string
					TargetPoolsAdded        TargetPoolsViewType
					TargetPoolsDeleted      TargetPoolsViewType
					InternalSettingsChanged []string
					ExternalSettingsChanged []string
				}
				Dynamic NodePoolsDiffResult
				Static  NodePoolsDiffResult

				PendingDynamicDeletions PendingDeletionsViewType
				PendingStaticDeletions  PendingDeletionsViewType
				RollingUpdate           PendingRollingUpdates
			}{}

			entry.CurrentIdx = oldIdx
			entry.DesiredIdx = idx
			entry.DNS = dnsChanged

			entry.Roles.Added = rolesAdded
			entry.Roles.Deleted = rolesDeleted
			entry.Roles.TargetPoolsAdded = targetPoolsAdded
			entry.Roles.TargetPoolsDeleted = targetPoolsDeleted
			entry.Roles.InternalSettingsChanged = internalSettingsChanged
			entry.Roles.ExternalSettingsChanged = externalSettingsChanged

			entry.PendingDynamicDeletions = pendingDynamicDeletions
			entry.PendingStaticDeletions = pendingStaticDeletions
			entry.RollingUpdate = rollingUpdates

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
	cid, did, change := determineLBApiEndpointChange(old.Clusters, desired.Clusters)
	result.ApiEndpoint.Current = cid
	result.ApiEndpoint.New = did
	result.ApiEndpoint.State = change

	return result
}

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
