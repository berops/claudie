package service

import (
	"fmt"
	"slices"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog/log"
)

// transferExistingState transfers existing data from current state to desired.
func transferExistingState(c *spec.Config) error {
	for cluster, state := range c.GetClusters() {
		log.Debug().Str("cluster", cluster).Msgf("reusing existing state")

		if err := transferExistingK8sState(state.GetCurrent().GetK8S(), state.GetDesired().GetK8S()); err != nil {
			return fmt.Errorf("error while updating Kubernetes cluster %q for config %s : %w", cluster, c.Name, err)
		}

		if err := transferExistingLBState(state.GetCurrent().GetLoadBalancers(), state.GetDesired().GetLoadBalancers()); err != nil {
			return fmt.Errorf("error while updating Loadbalancer cluster %q for config %s : %w", cluster, c.Name, err)
		}
	}

	return nil
}

// deduplicateNodepoolNames renames multiple references of the same nodepool in k8s,lb clusters to treat
// them as individual nodepools.
func deduplicateNodepoolNames(from *manifest.Manifest, state *spec.ClusterState) {
	desired := state.GetDesired().GetK8S()
	desiredLbs := state.GetDesired().GetLoadBalancers().GetClusters()

	current := state.GetCurrent().GetK8S()
	currentLbs := state.GetCurrent().GetLoadBalancers().GetClusters()

	for _, np := range from.NodePools.Dynamic {
		used := make(map[string]struct{})

		copyK8sNodePoolsNamesFromCurrentState(used, np.Name, current, desired)
		copyLbNodePoolNamesFromCurrentState(used, np.Name, currentLbs, desiredLbs)

		references := findNodePoolReferences(np.Name, desired.GetClusterInfo().GetNodePools())
		for _, lb := range desiredLbs {
			references = append(references, findNodePoolReferences(np.Name, lb.GetClusterInfo().GetNodePools())...)
		}

		for _, np := range references {
			hash := utils.CreateHash(utils.HashLength)
			for _, ok := used[hash]; ok; {
				hash = utils.CreateHash(utils.HashLength)
			}
			used[hash] = struct{}{}
			np.Name += fmt.Sprintf("-%s", hash)
		}
	}
}

// copyLbNodePoolNamesFromCurrentState copies the generated hash from an existing reference in the current state to the desired state.
func copyLbNodePoolNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired []*spec.LBcluster) {
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
				_, hash := utils.GetNameAndHashFromNodepool(nodepool, np.Name)
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
		_, hash := utils.GetNameAndHashFromNodepool(nodepool, np.Name)
		if hash == "" {
			continue
		}

		used[hash] = struct{}{}

		if np.IsControl && control != nil {
			control.Name += fmt.Sprintf("-%s", hash)
		} else if !np.IsControl && compute != nil {
			compute.Name += fmt.Sprintf("-%s", hash)
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

// transferExistingK8sState updates the desired state of the kubernetes clusters based on the current state
func transferExistingK8sState(current, desired *spec.K8Scluster) error {
	if desired == nil || current == nil {
		return nil
	}

	if err := transferNodePools(desired.ClusterInfo, current.ClusterInfo); err != nil {
		return err
	}

	if current.Kubeconfig != "" {
		desired.Kubeconfig = current.Kubeconfig
	}

	return nil
}

// transferDynamicNp updates the desired state of the kubernetes clusters based on the current state
// transferring only nodepoolData.
func transferDynamicNp(clusterID string, current, desired *spec.NodePool, updateAutoscaler bool) bool {
	dnp := desired.GetDynamicNodePool()
	cnp := current.GetDynamicNodePool()

	canUpdate := dnp != nil && cnp != nil
	if !canUpdate {
		return false
	}

	dnp.PublicKey = cnp.PublicKey
	dnp.PrivateKey = cnp.PrivateKey
	dnp.Cidr = cnp.Cidr

	if updateAutoscaler && dnp.AutoscalerConfig != nil {
		switch {
		case dnp.AutoscalerConfig.Min > cnp.Count:
			dnp.Count = dnp.AutoscalerConfig.Min
		case dnp.AutoscalerConfig.Max < cnp.Count:
			dnp.Count = dnp.AutoscalerConfig.Max
		default:
			dnp.Count = cnp.Count
		}
	}

	fillDynamicNodes(clusterID, current, desired)
	return true
}

// updateClusterInfo updates the desired state based on the current state
// clusterInfo.
func transferNodePools(desired, current *spec.ClusterInfo) error {
	desired.Hash = current.Hash
desired:
	for _, desiredNp := range desired.NodePools {
		for _, currentNp := range current.NodePools {
			if desiredNp.Name != currentNp.Name {
				continue
			}

			switch {
			case transferDynamicNp(utils.GetClusterID(desired), currentNp, desiredNp, true):
			case transferStaticNodes(utils.GetClusterID(desired), currentNp, desiredNp):
			default:
				return fmt.Errorf("%q is neither dynamic nor static, unexpected value: %v", desiredNp.Name, desiredNp.GetNodePoolType())
			}

			continue desired
		}
	}
	return nil
}

func fillDynamicNodes(clusterID string, current, desired *spec.NodePool) {
	dnp := desired.GetDynamicNodePool()

	nodes := make([]*spec.Node, 0, dnp.Count)
	nodeNames := make(map[string]struct{}, dnp.Count)

	for i, node := range current.Nodes {
		if i == int(dnp.Count) {
			break
		}
		nodes = append(nodes, node)
		nodeNames[node.Name] = struct{}{}
		log.Debug().Str("cluster", clusterID).Msgf("reusing node %q from current state nodepool %q, IsControl: %v, into desired state of the nodepool", node.Name, desired.Name, desired.IsControl)
	}

	nodepoolID := fmt.Sprintf("%s-%s", clusterID, desired.Name)
	for len(nodes) < int(dnp.Count) {
		name := uniqueNodeName(nodepoolID, nodeNames)
		nodeNames[name] = struct{}{}
		nodes = append(nodes, &spec.Node{Name: name})
		log.Debug().Str("cluster", clusterID).Msgf("adding node %q into desired state nodepool %q, IsControl: %v", name, desired.Name, desired.IsControl)
	}

	desired.Nodes = nodes
}

// uniqueNodeName returns new node name, which is guaranteed to be unique, based on the provided existing names.
func uniqueNodeName(nodepoolID string, existingNames map[string]struct{}) string {
	index := uint8(1)
	for {
		candidate := fmt.Sprintf("%s-%02x", nodepoolID, index)
		if _, ok := existingNames[candidate]; !ok {
			return candidate
		}
		index++
	}
}

func transferStaticNodes(clusterID string, current, desired *spec.NodePool) bool {
	dsp := desired.GetStaticNodePool()
	csp := current.GetStaticNodePool()

	canUpdate := dsp != nil && csp != nil
	if !canUpdate {
		return false
	}

	usedNames := make(map[string]struct{})
	for _, cn := range current.Nodes {
		usedNames[cn.Name] = struct{}{}
	}

	for _, dn := range desired.Nodes {
		i := slices.IndexFunc(current.Nodes, func(cn *spec.Node) bool { return cn.Public == dn.Public })
		if i < 0 {
			dn.Name = uniqueNodeName(desired.Name, usedNames)
			usedNames[dn.Name] = struct{}{}
			log.Debug().Str("cluster", clusterID).Msgf("adding static node %q into desired state nodepool %q, IsControl: %v", dn.Name, desired.Name, desired.IsControl)
			continue
		}
		dn.Name = current.Nodes[i].Name
		dn.Private = current.Nodes[i].Private
		dn.NodeType = current.Nodes[i].NodeType
		log.Debug().Str("cluster", clusterID).Msgf("reusing node %q from current state static nodepool %q, IsControl: %v, into desired state static nodepool", dn.Name, desired.Name, desired.IsControl)
	}

	return true
}

// transferExistingLBState updates the desired state of the loadbalancer clusters based on the current state
func transferExistingLBState(current, desired *spec.LoadBalancers) error {
	transferExistingDns(current, desired)

	currentLbs := current.GetClusters()
	desiredLbs := desired.GetClusters()

	for _, desired := range desiredLbs {
		for _, current := range currentLbs {
			if current.ClusterInfo.Name != desired.ClusterInfo.Name {
				continue
			}

			if err := transferNodePools(desired.ClusterInfo, current.ClusterInfo); err != nil {
				return err
			}

			break
		}
	}

	return nil
}

func transferExistingDns(current, desired *spec.LoadBalancers) {
	const hostnameHashLength = 17

	currentLbs := make(map[string]*spec.LBcluster)
	for _, cluster := range current.GetClusters() {
		currentLbs[cluster.ClusterInfo.Name] = cluster
	}

	for _, cluster := range desired.GetClusters() {
		previous, ok := currentLbs[cluster.ClusterInfo.Name]
		if !ok {
			if cluster.Dns.Hostname == "" {
				cluster.Dns.Hostname = utils.CreateHash(hostnameHashLength)
			}
			continue
		}
		if cluster.Dns.Hostname != "" {
			if previous.Dns.Hostname == cluster.Dns.Hostname {
				cluster.Dns.Endpoint = previous.Dns.Endpoint
			}
			continue
		}

		// copy hostname from current state if not specified in manifest
		if cluster.Dns.Hostname == "" || cluster.Dns.Endpoint == "" {
			cluster.Dns.Hostname = previous.Dns.Hostname
			cluster.Dns.Endpoint = previous.Dns.Endpoint
		}
	}
}