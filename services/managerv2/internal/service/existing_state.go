package service

import (
	"fmt"
	"slices"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"google.golang.org/protobuf/proto"
)

// TODO: fix me ip
// func backwardsCompatibilityTransferMissingState(c *spec.Config) {
// 	for _, state := range c.GetClusters() {
// 		for _, current := range state.GetCurrent().GetLoadBalancers().GetClusters() {
// 			// TODO: remove in future versions, cloudflare account id may not be correctly
// 			// propagated to the current state when upgrading claudie versions, since the
// 			// [manifest.Cloudflare.AccountID] has a validation which requires the presence
// 			// of a valid, non-empty `account_id`, which might be missing in the current state,
// 			// that will result in errors on subsequent workflows, simply transfer the `account_id`
// 			// from the desired state to the current state, only if it's empty. That will take care
// 			// of the drift introduced during claudie updates.
// 			if cc := current.GetDns().GetProvider().GetCloudflare(); cc != nil && cc.AccountID == "" {
// 				i := clusters.IndexLoadbalancerById(current.GetClusterInfo().Id(), state.GetDesired().GetLoadBalancers().GetClusters())
// 				if i >= 0 {
// 					dlb := state.Desired.LoadBalancers.Clusters[i]
// 					if dc := dlb.GetDns().GetProvider().GetCloudflare(); dc != nil {
// 						log.
// 							Info().
// 							Str("cluster", current.GetClusterInfo().Id()).
// 							Msg("detected drift in current state for Cloudflare AccountID, transferring state from desired state")
// 						cc.AccountID = dc.AccountID
// 					}
// 				}
// 			}
// 		}
// 	}
// }

func backwardsCompatibility(c *spec.Config) {
	// // TODO: remove in future versions, currently only for backwards compatibility.
	// // version 0.9.3 moved selection of the api server to the manager service
	// // and introduced a new field that selects which LB is used as the api endpoint.
	// // To have backwards compatibility with clusters build with versions before 0.9.3
	// // select the first load balancer in the current state and set this new field to true.
	// for _, state := range c.GetClusters() {
	// 	currentLbs := state.GetCurrent().GetLoadBalancers().GetClusters()
	// 	var (
	// 		anyApiServerLoadBalancerSelected bool
	// 		apiServerLoadBalancers           []int
	// 	)

	// 	for _, current := range currentLbs {
	// 		// TODO: remove in future versions, currently only for backwards compatibility.
	// 		// version 0.9.7 introced additional role settings, which may not be set in the
	// 		// current state. To have backwards compatibility add defaults to the current state.
	// 		for _, role := range current.Roles {
	// 			if role.Settings == nil {
	// 				log.Info().
	// 					Str("cluster", current.GetClusterInfo().Id()).
	// 					Msg("detected loadbalancer build with version older than 0.9.7, settings default role settings for its current state")

	// 				role.Settings = &spec.Role_Settings{
	// 					ProxyProtocol: true,
	// 				}
	// 			}
	// 		}
	// 	}

	// 	for i, current := range currentLbs {
	// 		if current.IsApiEndpoint() {
	// 			anyApiServerLoadBalancerSelected = true
	// 			break
	// 		}
	// 		if current.HasApiRole() && !current.UsedApiEndpoint && current.Dns != nil {
	// 			apiServerLoadBalancers = append(apiServerLoadBalancers, i)
	// 		}
	// 	}
	// 	if !anyApiServerLoadBalancerSelected && len(apiServerLoadBalancers) > 0 {
	// 		currentLbs[apiServerLoadBalancers[0]].UsedApiEndpoint = true
	// 		log.Info().
	// 			Str("cluster", currentLbs[apiServerLoadBalancers[0]].GetClusterInfo().Id()).
	// 			Msgf("detected api-server loadbalancer build with claudie version older than 0.9.3, selecting as the loadbalancer for the api-server")
	// 	}
	// }
}

// Finds matching nodepools in desired that are also in current and that both have nodepool type of
// [spec.NodePool_StaticNodePool]. For all of the nodes within the nodepool, Nodes with matching Public
// Endpoints (IP addresses), are ignored in this function as they have an already existing Name in the
// `current` state that needs to be transfered. For nodes that do not match unique names are generated
// such that they do no collide with the already assigned names in the `current` state. This function
// should be used in combination with [transferImmutableState] to both deduplicate the names
// and then transfer existing state.
//
// When used without the function just generates unique names for new Nodes that are not in `current`
// so the `desired` state for static nodepools will be in "Intermediate State", until the existing
// state is transferred to `desired`.
func deduplicateStaticNodeNames(current, desired *spec.Clusters) {
outer:
	for _, current := range current.GetK8S().GetClusterInfo().GetNodePools() {
		for _, desired := range desired.GetK8S().GetClusterInfo().GetNodePools() {
			if current.Name != desired.Name {
				continue
			}

			switch current.GetType().(type) {
			case *spec.NodePool_StaticNodePool:
				if desired.GetStaticNodePool() == nil {
					// names match, but not nodepool types.
					// Since there cannot be two nodepools
					// with the same name, break.
					continue outer
				}
			default:
				continue outer
			}

			usedNames := make(map[string]struct{})
			for _, n := range current.Nodes {
				usedNames[n.Name] = struct{}{}
			}

			// for all new nodes within the nodepool that also exists
			// in the current state, regenerate node names to be unique
			// and do not collide with the names in the current state.
			//
			// If the names of the nodes that exist in both current and desired,
			// based on a public endpoint match, do not match this should not be
			// handled in this function and should instead be handled with [transferStaticNodePool]
			// which trasnfers also the Name, that is considered "Immutable" once assigned.
			for _, n := range desired.Nodes {
				filter := func(cn *spec.Node) bool { return cn.Public == n.Public }
				if slices.IndexFunc(current.Nodes, filter) >= 0 {
					continue
				}
				n.Name = uniqueNodeName(desired.Name, usedNames)
			}
			break
		}
	}
}

// deduplicateDynamicNodePoolNames goes over each reference of each dynamic nodepool definition inside of
// [manifest.Manifest] and with the passed in `current` state deduplicates the names in the `desired` state
// by appending hashes to the names of the nodepools, in both k8s and loadbalancers. If `current` is empty
// no transferring is done and the dynamic nodepools in `desired` are simply assigned a randomly generated
// hash to their names.
func deduplicateDynamicNodePoolNames(from *manifest.Manifest, current, desired *spec.Clusters) {
	desiredK8s := desired.GetK8S()
	desiredLbs := desired.GetLoadBalancers().GetClusters()

	currentK8s := current.GetK8S()
	currentLbs := current.GetLoadBalancers().GetClusters()

	for _, np := range from.NodePools.Dynamic {
		used := make(map[string]struct{})

		copyK8sNodePoolsNamesFromCurrentState(used, np.Name, currentK8s, desiredK8s)
		copyLbNodePoolNamesFromCurrentState(used, np.Name, currentLbs, desiredLbs)

		references := nodepools.FindReferences(np.Name, desiredK8s.GetClusterInfo().GetNodePools())
		for _, lb := range desiredLbs {
			references = append(references, nodepools.FindReferences(np.Name, lb.GetClusterInfo().GetNodePools())...)
		}

		for _, np := range references {
			h := hash.Create(hash.Length)
			for _, ok := used[h]; ok; {
				h = hash.Create(hash.Length)
			}
			used[h] = struct{}{}
			np.Name += fmt.Sprintf("-%s", h)
		}
	}
}

// copyLbNodePoolNamesFromCurrentState copies the generated hash from an existing reference in the current state to the desired state.
func copyLbNodePoolNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired []*spec.LBcluster) {
	for _, desired := range desired {
		references := nodepools.FindReferences(nodepool, desired.GetClusterInfo().GetNodePools())
		switch {
		case len(references) > 1:
			panic("unexpected nodepool reference count")
		case len(references) == 0:
			continue
		}

		ref := references[0]

		for _, current := range current {
			if desired.ClusterInfo.Name != current.ClusterInfo.Name {
				continue
			}

			for _, np := range current.GetClusterInfo().GetNodePools() {
				_, hash := nodepools.MatchNameAndHashWithTemplate(nodepool, np.Name)
				if hash == "" {
					continue
				}

				used[hash] = struct{}{}

				ref.Name += fmt.Sprintf("-%s", hash)
				break
			}
		}
	}
}

// copyK8sNodePoolsNamesFromCurrentState copies the generated hash from an existing reference in the current state to the desired state.
func copyK8sNodePoolsNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired *spec.K8Scluster) {
	references := nodepools.FindReferences(nodepool, desired.GetClusterInfo().GetNodePools())

	switch {
	case len(references) == 0:
		return
	case len(references) > 2:
		// cannot reuse the same nodepool more than twice (once for control, once for worker pools)
		panic("unexpected nodepool reference count")
	}

	// to avoid extra code for special cases where there is just 1 reference, append a nil.
	references = append(references, []*spec.NodePool{nil}...)

	control, compute := references[0], references[1]
	if !references[0].IsControl {
		control, compute = compute, control
	}

	for _, np := range current.GetClusterInfo().GetNodePools() {
		_, hash := nodepools.MatchNameAndHashWithTemplate(nodepool, np.Name)
		if hash == "" {
			continue
		}

		used[hash] = struct{}{}

		if np.IsControl && control != nil {
			// if there are multiple nodepools in the current state (for example on a failed rolling update)
			// transfer only one of them.
			if _, h := nodepools.MatchNameAndHashWithTemplate(nodepool, control.Name); h == "" {
				control.Name += fmt.Sprintf("-%s", hash)
			}
		} else if !np.IsControl && compute != nil {
			// if there are multiple nodepools in the current state (for example on a failed rolling update)
			// transfer only one of them.
			if _, h := nodepools.MatchNameAndHashWithTemplate(nodepool, compute.Name); h == "" {
				compute.Name += fmt.Sprintf("-%s", hash)
			}
		}
	}
}

// transferImmutableState transfers state that was generated as part of [createDesiredState] or as part of the
// build pipeline of the cluster, and that once is generated/assigned, must stay unchanged even for the newly
// generated `desired` state.
//
// If the passed in `current` state is nil, the function panics as it expects to work with the current state
// and it is up to the caller to make sure the current state exists.
func transferImmutableState(current, desired *spec.Clusters) {
	transferK8sState(current.K8S, desired.K8S)
	transferLBState(current.LoadBalancers, desired.LoadBalancers)
}

// transferK8sState transfers only the kubernetes cluster relevant parts of `current` into `desired`.
// The function transfer only the state that should be "Immutable" once assigned to the kubernetes state.
func transferK8sState(current, desired *spec.K8Scluster) {
	transferClusterInfo(current.ClusterInfo, desired.ClusterInfo)
	desired.Kubeconfig = current.Kubeconfig

	// For now consider the network range for the VPN immutable as well, might change in the future.
	desired.Network = current.Network
}

// transferDynamicNodePool transfers state that should be "Immutable" from the
// `current` into the `desired` state. Immutable state must stay unchanged once
// it is assigned to a dynamic NodePool. Note that nodes that are assigned
// to the current state are immutable once build, but may be destroyed on changes
// to the desired state, but that is not handled here.
func transferDynamicNodePool(current, desired *spec.NodePool) {
	cnp := current.GetDynamicNodePool()
	dnp := desired.GetDynamicNodePool()

	dnp.PublicKey = cnp.PublicKey
	dnp.PrivateKey = cnp.PrivateKey
	dnp.Cidr = cnp.Cidr

	// Nodes in dynamic nodepools are not matched like how Static Nodepools are.
	// Within dynamic nodepools IP addresses can be recycled, so they can identify
	// different nodes, not necessarily the same node. Thus any nodes in the
	// desired state are cleared and the nodes of the current state are copied over
	// to the desired state.
	clear(desired.Nodes)
	desired.Nodes = desired.Nodes[:0]

	// To correctly transfer the existing state into the desired state three things
	// needs to be considered:
	//
	// 1. The target Size of Autoscaled nodepools.
	//    The target Size of Autoscaled is not managed by the InputManifest, so anything
	//    that is in the desired state is not the actuall target Size, with the expection
	//    when there is not current state, but in this function the expecation is that
	//    the current state exists.
	//
	//    The targetSize is a desired state that is externally managed and neither the
	//    manager nor the InputManifest state its desired value. The `cluster-autoscaler`
	//    pod specifies this and updates the current state within the manager, Thus the
	//    current State `TargetSize` actually states what the current desired capacity
	//    of the autoscaled nodepool is.
	if dnp.AutoscalerConfig != nil {
		if cnp.AutoscalerConfig != nil {
			switch {
			case dnp.AutoscalerConfig.Min > cnp.AutoscalerConfig.TargetSize:
				dnp.AutoscalerConfig.TargetSize = dnp.AutoscalerConfig.Min
			case dnp.AutoscalerConfig.Max < cnp.AutoscalerConfig.TargetSize:
				dnp.AutoscalerConfig.TargetSize = dnp.AutoscalerConfig.Max
			default:
				dnp.AutoscalerConfig.TargetSize = cnp.AutoscalerConfig.TargetSize
			}
		}

		// To resolve the actuall desired count of the nodepool in the desired state
		// consider the TargetSize of the nodepool.
		//
		// If the TargetSize is higher than what's currently in the cluster match the
		// targetSize as this will indicate (TargetSize - cnp.Count) new nodes are needed
		// in the desired state. Otherwise simply keep whatever is in the current state
		// and clamp it within the new [Min, Max] range.
		desiredCount := max(cnp.Count, dnp.AutoscalerConfig.TargetSize)

		switch {
		case dnp.AutoscalerConfig.Min > desiredCount:
			dnp.Count = dnp.AutoscalerConfig.Min
		case dnp.AutoscalerConfig.Max < desiredCount:
			dnp.Count = dnp.AutoscalerConfig.Max
		default:
			dnp.Count = desiredCount
		}
	}

	// 2. The count of both of the nodepools.
	// 	  Whatever the counts are, transfering the current number of
	//    nodes is limited by the minimum count of the possible nodes
	//    within the nodepools of both states.
	count := min(cnp.Count, dnp.Count)

	// 3. While transfering nodes from the current state
	//    consider ignoring nodes that were marked for
	//    deletion. This node status indicates that at
	//    some point in the future the nodes should be
	//    deleted, and in here since we are transfering
	//    existing state, we decide to also ignore nodes
	//    that are MarkedForDeletion and omitting them
	//    to be transferred to the desired state, which
	//    when diffed should trigger a deletion of those
	//    nodes, but **only if the count in the desired state is
	//    less than the count in the current state, as
	//    with this we have some place to get rid of some
	//    or possibly all nodes marked for deletion, while
	//    keeping running ok nodes within the 'count' range
	//    of the desired nodepool nodes.**, otherwise simply
	//    also transfer the MarkedForDeletion nodes into the
	//    desired state as they're running ok, and other parts
	//    of the code should at some point schedule it for
	//    deletion.
	//
	// Arithmetic cannot underflow here as the count of the
	// nodepool is limited to 255.
	//
	// If the `count` is `cnp.Count` then this will be 0.
	// indicating that the desired state of the nodepool wants
	// more nodes than there currently is, so we also keep
	// the MarkedForDeletion in here at this stage.
	//
	// If the `count` is `dnp.Count` then this will be a
	// positive number, indicating that the desired state
	// of the nodepool has fewer number of nodes than the
	// current state of the nodepool and we can decide at
	// this point which nodes we omit transferring and the
	// number below gives us how many nodes we can ommit altogether
	// but we interpret that number as the maximum amount of
	// MarkedForDeletion nodes that we can omit transferring
	// while trying to keep the other nodes in the desired
	// state. If there are more nodes that are MarkedForDeletion
	// not all of them will be omitted at this stage.
	skipMarkedForDeletion := max(cnp.Count-count, 0)

	for _, node := range current.Nodes[:count] {
		canSkip := skipMarkedForDeletion > 0
		canSkip = canSkip && node.Status == spec.NodeStatus_MarkedForDeletion
		if canSkip {
			skipMarkedForDeletion -= 1
			continue
		}

		n := proto.Clone(node).(*spec.Node)
		desired.Nodes = append(desired.Nodes, n)
	}
}

// transferStaticNodePool transfers state that should be "Immutable" from the
// `current` into the `desired` state. Immutable state must stay unchanged once
// it is assigned to a dynamic NodePool.
func transferStaticNodePool(current, desired *spec.NodePool) {
	for _, cn := range current.Nodes {
		for _, dn := range desired.Nodes {
			// Static nodes are identified based on the Publicly reachable IP.
			//
			// In here for the static nodes the check for the MarkedForDeletion
			// is not done, as static nodes are handled differently contrary to
			// the dynamic node.
			//
			// If the static node is found in the desired NodePool even if it
			// was marked for deletion it would be immediately rejoin as it
			// exists in the desired state, thus the logic around MarkedForDeletion
			// would be a Noop in here. Static nodes can still be MarkedForDeletion
			// the status will simply not be considered at this stage.
			if cn.Public != dn.Public {
				continue
			}

			dn.Name = cn.Name
			dn.Private = cn.Private
			dn.NodeType = cn.NodeType

			// Note: The [spec.NodePoolStatic.Keys] of the desired state do not need to
			// be updated here, as the key is not immutable and could be changed.
			// The desired state will already have the entry in the map populated for
			// the Public Endpoint we are matching against.
			break
		}
	}
}

// transferClusterInfo transfers state that should be "Immutable" once assigned
// from the `current` into the `desired` state.
func transferClusterInfo(current, desired *spec.ClusterInfo) {
	desired.Name = current.Name
	desired.Hash = current.Hash

outer:
	for _, desired := range desired.NodePools {
		for _, current := range current.NodePools {
			if current.Name != desired.Name {
				continue
			}

			switch current.GetType().(type) {
			case *spec.NodePool_DynamicNodePool:
				if desired.GetDynamicNodePool() == nil {
					// name match but not types.
					// Thus, no transferring can be done since
					// there cannot be two nodepools with the same
					// name.
					continue outer
				}
				transferDynamicNodePool(current, desired)
			case *spec.NodePool_StaticNodePool:
				if desired.GetStaticNodePool() == nil {
					// name match but not types.
					// Thus, no transferring can be done since
					// there cannot be two nodepools with the same
					// name.
					continue outer
				}
				transferStaticNodePool(current, desired)
			}
		}
	}
}

// transferLBState transfers the relevant parts of the `current` into `desired`.
// The function transfer only the state that should be "Immutable" once assigned.
func transferLBState(current, desired *spec.LoadBalancers) {
	for _, current := range current.GetClusters() {
		for _, desired := range desired.GetClusters() {
			if current.ClusterInfo.Name != desired.ClusterInfo.Name {
				continue
			}

			transferDns(current, desired)
			transferClusterInfo(current.ClusterInfo, desired.ClusterInfo)
			transferRoles(current.Roles, desired.Roles)
			desired.UsedApiEndpoint = current.UsedApiEndpoint
			break
		}
	}
}

func transferRoles(current, desired []*spec.Role) {
	for _, current := range current {
		for _, desired := range desired {
			if current.Name == desired.Name {
				desired.Settings.EnvoyAdminPort = current.Settings.EnvoyAdminPort
			}
		}
	}
}

func transferDns(current, desired *spec.LBcluster) {
	if current.Dns == nil {
		return
	}

	// transfer alternatives names.
	for _, current := range current.Dns.AlternativeNames {
		for _, desired := range desired.Dns.AlternativeNames {
			if desired.Hostname == current.Hostname {
				desired.Endpoint = current.Endpoint
			}
		}
	}

	// transfer the endpoint if the hostname did not change.
	if desired.Dns.Hostname != "" {
		if desired.Dns.Hostname == current.Dns.Hostname {
			desired.Dns.Endpoint = current.Dns.Endpoint
		}
		return
	}

	desired.Dns.Hostname = current.Dns.Hostname
	desired.Dns.Endpoint = current.Dns.Endpoint
}
