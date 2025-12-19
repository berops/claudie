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

// TODO: determin if we still need this with the reconciliation loop,
// probably not.
func backwardsCompatibility(c *spec.Config) {
	// 	// TODO: remove in future versions, currently only for backwards compatibility.
	// 	// version 0.9.3 moved selection of the api server to the manager service
	// 	// and introduced a new field that selects which LB is used as the api endpoint.
	// 	// To have backwards compatibility with clusters build with versions before 0.9.3
	// 	// select the first load balancer in the current state and set this new field to true.
	// 	for _, state := range c.GetClusters() {
	// 		currentLbs := state.GetCurrent().GetLoadBalancers().GetClusters()
	// 		var (
	// 			anyApiServerLoadBalancerSelected bool
	// 			apiServerLoadBalancers           []int
	// 		)

	// 		for i, current := range currentLbs {
	// 			// TODO: remove in future versions, currently only for backwards compatibility.
	// 			// version 0.9.7 introced additional role settings, which may not be set in the
	// 			// current state. To have backwards compatibility add defaults to the current state.
	// 			for _, role := range current.Roles {
	// 				if role.Settings == nil {
	// 					log.Info().
	// 						Str("cluster", current.GetClusterInfo().Id()).
	// 						Msg("detected loadbalancer build with version older than 0.9.7, settings default role settings for its current state")

	// 					role.Settings = &spec.Role_Settings{
	// 						ProxyProtocol: true,
	// 					}
	// 				}
	// 			}

	//				if current.IsApiEndpoint() {
	//					anyApiServerLoadBalancerSelected = true
	//					break
	//				}
	//				if current.HasApiRole() && !current.UsedApiEndpoint && current.Dns != nil {
	//					apiServerLoadBalancers = append(apiServerLoadBalancers, i)
	//				}
	//			}
	//			if !anyApiServerLoadBalancerSelected && len(apiServerLoadBalancers) > 0 {
	//				currentLbs[apiServerLoadBalancers[0]].UsedApiEndpoint = true
	//				log.Info().
	//					Str("cluster", currentLbs[apiServerLoadBalancers[0]].GetClusterInfo().Id()).
	//					Msgf("detected api-server loadbalancer build with claudie version older than 0.9.3, selecting as the loadbalancer for the api-server")
	//			}
	//		}
	//	}
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
	// TODO: what happens when I change the [spec.K8SclusterV2.Network] ?
	// I think that should stay immutable as well... or maybe not ?
	transferClusterInfo(current.ClusterInfo, desired.ClusterInfo)
	desired.Kubeconfig = current.Kubeconfig
}

// transferDynamicNodePool transfers state that should be "Immutable" from the
// `current` into the `desired` state. Immutable state must stay unchanged once
// it is assigned assigned to a dynamic NodePool. Note that nodes that are assigned
// to the current state are immutable once build, but may be destroyed on changes
// to the desired state, but that is not handled here.
func transferDynamicNodePool(current, desired *spec.NodePool) {
	cnp := current.GetDynamicNodePool()
	dnp := desired.GetDynamicNodePool()

	dnp.PublicKey = cnp.PublicKey
	dnp.PrivateKey = cnp.PrivateKey
	dnp.Cidr = cnp.Cidr

	clear(desired.Nodes)
	desired.Nodes = desired.Nodes[:0]

	// Nodes in dynamic nodepools are not matched like how Static Nodepools are.
	// Within dynamic nodepools IP addresses can be recycled, so they can identify
	// different nodes, not necessarily the same node. Thus any nodes in the
	// desired state are cleared and the nodes of the current state are copied over
	// to the desired state.
	count := min(cnp.Count, dnp.Count)
	for _, node := range current.Nodes[:count] {
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
