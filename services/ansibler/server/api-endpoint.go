package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/ansible"
	"github.com/berops/claudie/services/ansibler/templates"
)

// updateAPIEndpoint handles the case where Control Node with ApiEndpoint where removed from
// the desired state and thus a new ApiEndpoint needs to be selected among the existing control nodes.
func updateAPIEndpoint(current, desired *pb.ClusterInfo) error {
	clusterID := fmt.Sprintf("%s-%s", current.Name, current.Hash)
	directory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, utils.CreateHash(utils.HashLength)))

	// check if the nodepool is present in the desired state
	// 	  if not, choose one of the remaining control nodes as the new api endpoint.
	apiEndpointNodePool, apiEndpointNode, err := utils.FindAPIEndpointNodePoolWithNode(current.GetNodePools())
	if err != nil {
		return fmt.Errorf("failed to find node with type: %s", pb.NodeType_apiEndpoint.String())
	}

	contains := utils.Contains(apiEndpointNodePool, desired.GetNodePools(), func(item *pb.NodePool, other *pb.NodePool) bool {
		return item.GetName() == other.GetName()
	})

	if !contains {
		if err := utils.CreateDirectory(directory); err != nil {
			return fmt.Errorf("failed to create directory %s : %w", directory, err)
		}

		if err := utils.CreateKeyFile(current.PrivateKey, directory, "k8s.pem"); err != nil {
			return fmt.Errorf("failed to create key file for %s : %w", clusterID, err)
		}

		// re-use the information for the LB cluster.
		err := generateInventoryFile(templates.LoadbalancerInventoryTemplate, directory, LbInventoryData{
			K8sNodepools: current.GetNodePools(),
			LBClusters:   nil,
			ClusterID:    clusterID,
		})

		if err != nil {
			return fmt.Errorf("error while creating inventory file for %s : %w", directory, err)
		}

		// find control nodepool present in both desired and current state.
		newNp, err := findNewAPIEndpointCandidate(current.GetNodePools(), desired.GetNodePools(), apiEndpointNodePool)
		if err != nil {
			return err
		}

		newEndpointNode := newNp.GetNodes()[0]

		// update the current state
		apiEndpointNode.NodeType = pb.NodeType_master
		newEndpointNode.NodeType = pb.NodeType_apiEndpoint

		if err := changeAPIEndpoint(current.Name, apiEndpointNode.GetPublic(), newEndpointNode.GetPublic(), directory); err != nil {
			return err
		}

		return os.RemoveAll(directory)
	}

	return nil
}

// changeAPIEndpoint will change kubeadm configuration to include new EP
func changeAPIEndpoint(clusterName, oldEndpoint, newEndpoint, directory string) error {
	ansible := ansible.Ansible{
		Playbook:  apiChangePlaybook,
		Inventory: inventoryFile,
		Flags:     fmt.Sprintf("--extra-vars \"NewEndpoint=%s OldEndpoint=%s\"", newEndpoint, oldEndpoint),
		Directory: directory,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("EP - %s", clusterName)); err != nil {
		return fmt.Errorf("error while running ansible: %w ", err)
	}

	return nil
}

// findNewAPIEndpointCandidate finds control plane nodepools present in both current (excluding the request nodepool)
// and desired state. Returns the first.
func findNewAPIEndpointCandidate(current, desired []*pb.NodePool, exclude *pb.NodePool) (*pb.NodePool, error) {
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
