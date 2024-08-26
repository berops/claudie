package service

import (
	"fmt"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/server/internal/store"
)

// transferExistingState transfers existing data from current state to desired.
func transferExistingState(m *manifest.Manifest, db *store.Config) error {
	// Since we're working with cluster states directly, we'll use the grpc form.
	// As in the DB form they're encoded.
	grpcRepr, err := store.ConvertToGRPC(db)
	if err != nil {
		return fmt.Errorf("failed to convert from db representation to grpc %q: %w", db.Name, err)
	}

	for cluster, state := range grpcRepr.GetClusters() {
		deduplicateNodepoolNames(m, state)

		if err := transferExistingK8sState(state.GetCurrent().GetK8S(), state.GetDesired().GetK8S()); err != nil {
			return fmt.Errorf("error while updating Kubernetes cluster %q for config %s : %w", cluster, db.Name, err)
		}

		if err := transferExistingLBState(state.GetCurrent().GetLoadBalancers(), state.GetDesired().GetLoadBalancers()); err != nil {
			return fmt.Errorf("error while updating Loadbalancer cluster %q for config %s : %w", cluster, db.Name, err)
		}
	}

	newdb, err := store.ConvertFromGRPC(grpcRepr)
	if err != nil {
		return fmt.Errorf("failed to convert from grpc to db representation %q: %w", grpcRepr.Name, err)
	}

	*db = *newdb

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
func transferExistingK8sState(current *spec.K8Scluster, desired *spec.K8Scluster) error {
	if desired == nil || current == nil {
		return nil
	}

	if err := updateClusterInfo(desired.ClusterInfo, current.ClusterInfo); err != nil {
		return err
	}

	if current.Kubeconfig != "" {
		desired.Kubeconfig = current.Kubeconfig
	}

	return nil
}

// updateClusterInfo updates the desired state based on the current state
// namely:
// - Hash
// - AutoscalerConfig
// - existing nodes
// - nodepool
//   - metadata
//   - Public key
//   - Private key
func updateClusterInfo(desired, current *spec.ClusterInfo) error {
	desired.Hash = current.Hash
desired:
	for _, desiredNp := range desired.NodePools {
		for _, currentNp := range current.NodePools {
			if desiredNp.Name != currentNp.Name {
				continue
			}

			switch {
			case tryUpdateDynamicNodePool(desiredNp, currentNp):
			case tryUpdateStaticNodePool(desiredNp, currentNp):
			default:
				return fmt.Errorf("%q is neither dynamic nor static, unexpected value: %v", desiredNp.Name, desiredNp.GetNodePoolType())
			}

			continue desired
		}
	}
	return nil
}

func tryUpdateDynamicNodePool(desired, current *spec.NodePool) bool {
	dnp := desired.GetDynamicNodePool()
	cnp := current.GetDynamicNodePool()

	canUpdate := dnp != nil && cnp != nil
	if !canUpdate {
		return false
	}

	dnp.PublicKey = cnp.PublicKey
	dnp.PrivateKey = cnp.PrivateKey

	desired.Nodes = current.Nodes
	dnp.Cidr = cnp.Cidr

	// Update the count
	if cnp.AutoscalerConfig != nil && dnp.AutoscalerConfig != nil {
		// Both have Autoscaler conf defined, use same count as in current
		dnp.Count = cnp.Count
	} else if cnp.AutoscalerConfig == nil && dnp.AutoscalerConfig != nil {
		// Desired is autoscaled, but not current
		if dnp.AutoscalerConfig.Min > cnp.Count {
			// Cannot have fewer nodes than defined min
			dnp.Count = dnp.AutoscalerConfig.Min
		} else if dnp.AutoscalerConfig.Max < cnp.Count {
			// Cannot have more nodes than defined max
			dnp.Count = dnp.AutoscalerConfig.Max
		} else {
			// Use same count as in current for now, autoscaler might change it later
			dnp.Count = cnp.Count
		}
	}

	return true
}

func tryUpdateStaticNodePool(desired, current *spec.NodePool) bool {
	dnp := desired.GetStaticNodePool()
	cnp := current.GetStaticNodePool()

	canUpdate := dnp != nil && cnp != nil
	if !canUpdate {
		return false
	}

	for _, dn := range desired.Nodes {
		for _, cn := range current.Nodes {
			if dn.Public == cn.Public {
				dn.Name = cn.Name
				dn.Private = cn.Private
				dn.NodeType = cn.NodeType
			}
		}
	}

	return true
}

// transferExistingLBState updates the desired state of the loadbalancer clusters based on the current state
func transferExistingLBState(current *spec.LoadBalancers, desired *spec.LoadBalancers) error {
	transferExistingDns(current, desired)

	currentLbs := current.GetClusters()
	desiredLbs := desired.GetClusters()

	for _, desired := range desiredLbs {
		for _, current := range currentLbs {
			if current.ClusterInfo.Name != desired.ClusterInfo.Name {
				continue
			}

			if err := updateClusterInfo(desired.ClusterInfo, current.ClusterInfo); err != nil {
				return err
			}

			break
		}
	}

	return nil
}

func transferExistingDns(current, desired *spec.LoadBalancers) {
	const hostnameHashLength = 17

	if desired == nil {
		return
	}

	currentLbs := make(map[string]*spec.LBcluster)

	for _, cluster := range current.GetClusters() {
		currentLbs[cluster.ClusterInfo.Name] = cluster
	}

	for _, cluster := range desired.GetClusters() {
		if previous, ok := currentLbs[cluster.ClusterInfo.Name]; !ok && cluster.Dns.Hostname == "" {
			cluster.Dns.Hostname = utils.CreateHash(hostnameHashLength)
		} else {
			// copy hostname from current state if not specified in manifest
			if cluster.Dns.Hostname == "" {
				cluster.Dns.Hostname = previous.Dns.Hostname
				cluster.Dns.Endpoint = previous.Dns.Endpoint
			}
		}
	}
}
