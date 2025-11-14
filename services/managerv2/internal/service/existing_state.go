package service

import (
	"fmt"
	"slices"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/proto"
)

func backwardsCompatibility(c *spec.ConfigV2) {
	// TODO: remove in future versions, currently only for backwards compatibility.
	// version 0.9.3 moved selection of the api server to the manager service
	// and introduced a new field that selects which LB is used as the api endpoint.
	// To have backwards compatibility with clusters build with versions before 0.9.3
	// select the first load balancer in the current state and set this new field to true.
	for _, state := range c.GetClusters() {
		currentLbs := state.GetCurrent().GetLoadBalancers().GetClusters()
		var (
			anyApiServerLoadBalancerSelected bool
			apiServerLoadBalancers           []int
		)

		for i, current := range currentLbs {
			// TODO: remove in future versions, currently only for backwards compatibility.
			// version 0.9.7 introced additional role settings, which may not be set in the
			// current state. To have backwards compatibility add defaults to the current state.
			for _, role := range current.Roles {
				if role.Settings == nil {
					log.Info().
						Str("cluster", current.GetClusterInfo().Id()).
						Msg("detected loadbalancer build with version older than 0.9.7, settings default role settings for its current state")

					role.Settings = &spec.RoleV2_Settings{
						ProxyProtocol: true,
					}
				}
			}

			if current.IsApiEndpoint() {
				anyApiServerLoadBalancerSelected = true
				break
			}
			if current.HasApiRole() && !current.UsedApiEndpoint && current.Dns != nil {
				apiServerLoadBalancers = append(apiServerLoadBalancers, i)
			}
		}
		if !anyApiServerLoadBalancerSelected && len(apiServerLoadBalancers) > 0 {
			currentLbs[apiServerLoadBalancers[0]].UsedApiEndpoint = true
			log.Info().
				Str("cluster", currentLbs[apiServerLoadBalancers[0]].GetClusterInfo().Id()).
				Msgf("detected api-server loadbalancer build with claudie version older than 0.9.3, selecting as the loadbalancer for the api-server")
		}
	}
}

// deduplicateStaticNodeNames re-assings all names for static nodes for the `desired` nodepools
// such that nodes with the same public IP will keep existing name and new nodes will have
// a unique not used name.
func deduplicateStaticNodeNames(current, desired *spec.ClustersV2) {
outer:
	for _, current := range current.GetK8S().GetClusterInfo().GetNodePools() {
		for _, desired := range desired.GetK8S().GetClusterInfo().GetNodePools() {
			if current.Name != desired.Name {
				continue
			}

			switch current.GetType().(type) {
			case *spec.NodePool_StaticNodePool:
				if desired.GetStaticNodePool() == nil {
					// names match but not nodepool types
					// since there cannot be two nodepools
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
			// If the names of the nodes that exist in both current and desired
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

// deduplicateDynamicNodepoolNames renames multiple references of the same dynamic nodepool in k8s
// and loadbalancer clusters to treat them as individual nodepools.
func deduplicateDynamicNodepoolNames(from *manifest.Manifest, current, desired *spec.ClustersV2) {
	desiredK8s := desired.GetK8S()
	desiredLbs := desired.GetLoadBalancers().GetClusters()

	currentK8s := current.GetK8S()
	currentLbs := current.GetLoadBalancers().GetClusters()

	for _, np := range from.NodePools.Dynamic {
		used := make(map[string]struct{})

		copyK8sNodePoolsNamesFromCurrentState(used, np.Name, currentK8s, desiredK8s)
		copyLbNodePoolNamesFromCurrentState(used, np.Name, currentLbs, desiredLbs)

		references := findNodePoolReferences(np.Name, desiredK8s.GetClusterInfo().GetNodePools())
		for _, lb := range desiredLbs {
			references = append(references, findNodePoolReferences(np.Name, lb.GetClusterInfo().GetNodePools())...)
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
func copyLbNodePoolNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired []*spec.LBclusterV2) {
	for _, desired := range desired {
		references := findNodePoolReferences(nodepool, desired.GetClusterInfo().GetNodePools())
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
func copyK8sNodePoolsNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired *spec.K8SclusterV2) {
	references := findNodePoolReferences(nodepool, desired.GetClusterInfo().GetNodePools())

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

// findNodePoolReferences find all nodepools that share the given name.
func findNodePoolReferences(name string, nodePools []*spec.NodePool) []*spec.NodePool {
	var references []*spec.NodePool
	for _, np := range nodePools {
		if np.Name == name {
			references = append(references, np)
		}
	}
	return references
}

// transferPreviouslyAcquiredState transfer state that was not generated as part of [createDesiredState]
// and parts that were generated but must stay unchanged even for the newly generated `desired` state.
func transferPreviouslyAcquiredState(current, desired *spec.ClustersV2) {
	transferK8sState(current.K8S, desired.K8S)
	transferLBState(current.LoadBalancers, desired.LoadBalancers)
}

// transferK8sState transfers only the kubernetes cluster relevant parts of the `current` into `desired`.
// The function transfer only the state that should be "Immutable" once assigned.
func transferK8sState(current, desired *spec.K8SclusterV2) {
	// TODO: what happens when I change the [spec.K8SclusterV2.Network] ?
	// I think that should stay immutable as well... or maybe not ?
	transferClusterInfo(current.ClusterInfo, desired.ClusterInfo)
	desired.Kubeconfig = current.Kubeconfig
}

// transferDynamicNodePool transfers state that should be "Immutable" once assigned
// from the `current` into the `desired` state.
func transferDynamicNodePool(current, desired *spec.NodePool) {
	cnp := current.GetDynamicNodePool()
	dnp := current.GetDynamicNodePool()

	dnp.PublicKey = cnp.PublicKey
	dnp.PrivateKey = cnp.PrivateKey
	dnp.Cidr = cnp.Cidr

	clear(desired.Nodes)
	desired.Nodes = desired.Nodes[:0]

	count := min(cnp.Count, dnp.Count)
	for _, node := range current.Nodes[:count] {
		n := proto.Clone(node).(*spec.Node)
		desired.Nodes = append(desired.Nodes, n)
	}
}

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

			// Note: The node Keys of the desired state do not need to
			// be updated here, as the key is not immutable and could be changed.

			break
		}
	}
}

// transferClusterInfo transfers state that should be "Immutable" once assigned
// from the `current` into the `desired` state.
func transferClusterInfo(current, desired *spec.ClusterInfoV2) {
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
func transferLBState(current, desired *spec.LoadBalancersV2) {
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

func transferRoles(current, desired []*spec.RoleV2) {
	for _, current := range current {
		for _, desired := range desired {
			if current.Name == desired.Name {
				desired.Settings.EnvoyAdminPort = current.Settings.EnvoyAdminPort
			}
		}
	}
}

func transferDns(current, desired *spec.LBclusterV2) {
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
