package usecases

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/ansible"
)

const (
	// TODO: change to appropriate path
	apiChangePlaybook = "../../ansible-playbooks/apiEndpointChange.yml"
)

func (u *Usecases) UpdateAPIEndpoint(request *pb.UpdateAPIEndpointRequest) (*pb.UpdateAPIEndpointResponse, error) {
	if request.Current == nil {
		return &pb.UpdateAPIEndpointResponse{Current: request.Current, Desired: request.Desired}, nil
	}

	log.Info().Msgf("Updating api endpoint for cluster %s project %s", request.Current.ClusterInfo.Name, request.ProjectName)
	if err := updateAPIEndpoint(request.Current.ClusterInfo, request.Desired.ClusterInfo); err != nil {
		return nil, fmt.Errorf("Failed to update api endpoint for cluster %s project %s", request.Current.ClusterInfo.Name, request.ProjectName)
	}
	log.Info().Msgf("Updated api endpoint for cluster %s project %s", request.Current.ClusterInfo.Name, request.ProjectName)

	return &pb.UpdateAPIEndpointResponse{Current: request.Current, Desired: request.Desired}, nil
}

// updateAPIEndpoint handles the case where the ApiEndpoint node is removed from
// the desired state. Thus a new control node needs to be selected among the existing
// control nodes. This new control node will then represent the ApiEndpoint of the cluster.
func updateAPIEndpoint(currentK8sClusterInfo, desiredK8sClusterInfo *pb.ClusterInfo) error {
	clusterID := fmt.Sprintf("%s-%s", currentK8sClusterInfo.Name, currentK8sClusterInfo.Hash)

	// Find the ApiEndpoint node from the current state of the K8s cluster.
	apiEndpointNodePool, apiEndpointNode, err := utils.FindNodepoolWithApiEndpointNode(currentK8sClusterInfo.GetNodePools())
	if err != nil {
		return fmt.Errorf("Failed to find the node with type: %s", pb.NodeType_apiEndpoint.String())
	}
	// Check whether that node still exists in the desired state of the cluster or not.
	apiEndpointNodeExists := utils.Contains(apiEndpointNodePool, desiredK8sClusterInfo.GetNodePools(),
		func(item *pb.NodePool, other *pb.NodePool) bool {
			return item.GetName() == other.GetName()
		},
	)

	if apiEndpointNodeExists {
		return nil
	}

	// If the ApiEndpoint type node doesn't exist

	// This is the directory where files (Ansible inventory files, SSH keys etc.) will be generated.
	outputDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, utils.CreateHash(utils.HashLength)))
	if err := utils.CreateDirectory(outputDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", outputDirectory, err)
	}

	// This SSH key is used by Ansible to SSH into the K8s cluster nodes.
	if err := utils.CreateKeyFile(currentK8sClusterInfo.PrivateKey, outputDirectory, "k8s.pem"); err != nil {
		return fmt.Errorf("failed to create key file for %s : %w", clusterID, err)
	}

	//
	err = ansible.GenerateInventoryFile(lbInventoryFileName, outputDirectory, LbInventoryFileParameters{
		K8sNodepools: currentK8sClusterInfo.GetNodePools(),
		LBClusters:   nil,
		ClusterID:    clusterID,
	})
	if err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", outputDirectory, err)
	}

	// // find control nodepool present in both desired and current state.
	// newNp, err := findNewAPIEndpointCandidate(currentK8sClusterInfo.GetNodePools(), desiredK8sClusterInfo.GetNodePools(), apiEndpointNodePool)
	// if err != nil {
	// 	return err
	// }

	// newEndpointNode := newNp.GetNodes()[0]

	// // update the current state
	// apiEndpointNode.NodeType = pb.NodeType_master
	// newEndpointNode.NodeType = pb.NodeType_apiEndpoint

	// if err := changeAPIEndpoint(currentK8sClusterInfo.Name, apiEndpointNode.GetPublic(), newEndpointNode.GetPublic(), outputDirectory); err != nil {
	// 	return err
	// }

	// Cleanup
	return os.RemoveAll(outputDirectory)
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

// changeAPIEndpoint will change the kubeadm configuration.
// It will set the Api endpoint of the cluster to the public IP of the
// newly selected ApiEndpoint node.
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

type LbInventoryFileParameters struct {
	K8sNodepools []*pb.NodePool
	LBClusters   []*pb.LBcluster
	ClusterID    string
}
