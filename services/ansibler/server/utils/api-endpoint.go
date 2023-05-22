package utils

import (
	"fmt"

	"github.com/berops/claudie/proto/pb"
)

const apiChangePlaybookFilePath = "../../ansible-playbooks/apiEndpointChange.yml"

// FindNewAPIEndpointCandidate finds control plane nodepools present in both current (excluding the request nodepool)
// and desired state. Returns the first.
func FindNewAPIEndpointCandidate(current, desired []*pb.NodePool, exclude *pb.NodePool) (*pb.NodePool, error) {
	currentPools := make(map[string]*pb.NodePool)
	for _, np := range current {
		if np.IsControl && np.Name != exclude.Name {
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
		Inventory: inventoryFileName,
		Flags:     fmt.Sprintf("--extra-vars \"NewEndpoint=%s OldEndpoint=%s\"", newEndpoint, oldEndpoint),
		Directory: directory,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("EP - %s", clusterName)); err != nil {
		return fmt.Errorf("error while running ansible: %w ", err)
	}

	return nil
}
