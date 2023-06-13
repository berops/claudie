package utils

import (
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

const apiChangePlaybookFilePath = "../../ansible-playbooks/apiEndpointChange.yml"

// FindNewAPIEndpointCandidate finds control plane nodepools present in both current (excluding the request nodepool)
// and desired state. Returns the first.
func FindNewAPIEndpointCandidate(current, desired []*pb.NodePool, excludeNodepoolName *pb.NodePool) (*pb.NodePool, error) {
	currentPools := make(map[string]*pb.NodePool)
	// Identify node pools in current state.
	for _, np := range current {
		if np.IsControl && np.Name != excludeNodepoolName.Name {
			currentPools[np.Name] = np
		}
	}

	for _, np := range desired {
		if np.IsControl {
			if candidate, ok := currentPools[np.Name]; ok {
				return candidate, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to find control plane nodepool present in both current and desired state")
}

// ChangeAPIEndpoint will change the kubeadm configuration.
// It will set the Api endpoint of the cluster to the public IP of the
// newly selected ApiEndpoint node.
func ChangeAPIEndpoint(clusterName, oldEndpoint, newEndpoint, directory string) error {
	ansible := Ansible{
		Playbook:  apiChangePlaybookFilePath,
		Inventory: InventoryFileName,
		Flags:     fmt.Sprintf("--extra-vars \"NewEndpoint=%s OldEndpoint=%s\"", newEndpoint, oldEndpoint),
		Directory: directory,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("EP - %s", clusterName)); err != nil {
		return fmt.Errorf("error while running ansible: %w ", err)
	}

	return nil
}

// FindCurrentAPIServerTypeLBCluster finds the current API server type LB cluster.
func FindCurrentAPIServerTypeLBCluster(lbClusters []*LBClusterData) *LBClusterData {
	for _, lbClusterData := range lbClusters {
		if lbClusterData.CurrentLbCluster != nil {
			if utils.HasAPIServerRole(lbClusterData.CurrentLbCluster.Roles) {
				return lbClusterData
			}
		}
	}

	return nil
}
