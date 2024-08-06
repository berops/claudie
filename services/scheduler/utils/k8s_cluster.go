package utils

import (
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
)

// CreateK8sCluster reads the unmarshalled manifest and creates desired state for Kubernetes clusters.
// Returns slice of *spec.K8Scluster if successful, nil otherwise
func CreateK8sCluster(unmarshalledManifest *manifest.Manifest) ([]*spec.K8Scluster, error) {
	var clusters []*spec.K8Scluster
	// Loop through clusters mentioned in the manifest
	for _, cluster := range unmarshalledManifest.Kubernetes.Clusters {
		// Generate variables
		newCluster := &spec.K8Scluster{
			ClusterInfo: &spec.ClusterInfo{
				Name: strings.ToLower(cluster.Name),
				Hash: utils.CreateHash(utils.HashLength),
			},
			Kubernetes: cluster.Version,
			Network:    cluster.Network,
		}

		// create node-pools
		controlNodePools, err := unmarshalledManifest.CreateNodepools(cluster.Pools.Control, true)
		if err != nil {
			return nil, fmt.Errorf("error while creating control nodepool for %s : %w", cluster.Name, err)
		}
		computeNodePools, err := unmarshalledManifest.CreateNodepools(cluster.Pools.Compute, false)
		if err != nil {
			return nil, fmt.Errorf("error while creating compute nodepool for %s : %w", cluster.Name, err)
		}

		newCluster.ClusterInfo.NodePools = append(controlNodePools, computeNodePools...)
		clusters = append(clusters, newCluster)
	}
	return clusters, nil
}

// UpdateK8sClusters updates the desired state of the kubernetes clusters based on the current state
// returns error if failed, nil otherwise
func UpdateK8sClusters(newConfig *spec.Config) error {
clusterDesired:
	for _, clusterDesired := range newConfig.DesiredState.Clusters {
		for _, clusterCurrent := range newConfig.CurrentState.Clusters {
			// Found current cluster with matching name
			if clusterDesired.ClusterInfo.Name == clusterCurrent.ClusterInfo.Name {
				if err := updateClusterInfo(clusterDesired.ClusterInfo, clusterCurrent.ClusterInfo); err != nil {
					return err
				}

				// create SSH keys for new nodepools that were added.
				if err := generateSSHKeys(clusterDesired.ClusterInfo); err != nil {
					return fmt.Errorf("error encountered while creating desired state for %s : %w", clusterDesired.ClusterInfo.Name, err)
				}

				if clusterCurrent.Kubeconfig != "" {
					clusterDesired.Kubeconfig = clusterCurrent.Kubeconfig
				}

				// Skip the checks bellow
				continue clusterDesired
			}
		}
		if err := generateSSHKeys(clusterDesired.ClusterInfo); err != nil {
			return fmt.Errorf("error encountered while creating desired state for %s : %w", clusterDesired.ClusterInfo.Name, err)
		}
	}
	return nil
}

// CopyLbNodePoolNamesFromCurrentState copies the generated hash from an existing reference in the current state to the desired state.
func CopyLbNodePoolNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired []*spec.LBcluster) {
	for _, desired := range desired {
		references := FindNodePoolReferences(nodepool, desired.GetClusterInfo().GetNodePools())
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

// CopyK8sNodePoolsNamesFromCurrentState copies the generated hash from an existing reference in the current state to the desired state.
func CopyK8sNodePoolsNamesFromCurrentState(used map[string]struct{}, nodepool string, current, desired *spec.K8Scluster) {
	references := FindNodePoolReferences(nodepool, desired.GetClusterInfo().GetNodePools())
	switch {
	case len(references) == 0:
		return
	case len(references) > 2:
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

// FindNodePoolReferences find all nodepools that share the given name.
func FindNodePoolReferences(name string, nodePools []*spec.NodePool) []*spec.NodePool {
	var references []*spec.NodePool
	for _, np := range nodePools {
		if np.Name == name {
			references = append(references, np)
		}
	}
	return references
}
