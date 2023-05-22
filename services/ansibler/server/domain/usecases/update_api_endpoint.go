package usecases

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/utils"
)

const (
	// TODO: change to appropriate path
	apiChangePlaybookFilePath = "../../ansible-playbooks/apiEndpointChange.yml"
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
	apiEndpointNodePool, apiEndpointNode, err := commonUtils.FindNodepoolWithApiEndpointNode(currentK8sClusterInfo.GetNodePools())
	if err != nil {
		return fmt.Errorf("Failed to find the node with type: %s", pb.NodeType_apiEndpoint.String())
	}
	// Check whether that node still exists in the desired state of the cluster or not.
	apiEndpointNodeExists := commonUtils.Contains(apiEndpointNodePool, desiredK8sClusterInfo.GetNodePools(),
		func(item *pb.NodePool, other *pb.NodePool) bool {
			return item.GetName() == other.GetName()
		},
	)

	if apiEndpointNodeExists {
		return nil
	}

	// If the ApiEndpoint type node doesn't exist

	// This is the directory where files (Ansible inventory files, SSH keys etc.) will be generated.
	outputDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, commonUtils.CreateHash(commonUtils.HashLength)))
	if err := commonUtils.CreateDirectory(outputDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", outputDirectory, err)
	}

	// This SSH key is used by Ansible to SSH into the K8s cluster nodes.
	if err := commonUtils.CreateKeyFile(currentK8sClusterInfo.PrivateKey, outputDirectory, "k8s.pem"); err != nil {
		return fmt.Errorf("failed to create key file for %s : %w", clusterID, err)
	}

	//
	err = utils.GenerateInventoryFile(utils.LBInventoryFileName, outputDirectory, utils.LbInventoryFileParameters{
		K8sNodepools: currentK8sClusterInfo.GetNodePools(),
		LBClusters:   nil,
		ClusterID:    clusterID,
	})
	if err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", outputDirectory, err)
	}

	// find control nodepool present in both desired and current state.
	newNp, err := utils.FindNewAPIEndpointCandidate(currentK8sClusterInfo.GetNodePools(), desiredK8sClusterInfo.GetNodePools(), apiEndpointNodePool)
	if err != nil {
		return err
	}

	newEndpointNode := newNp.GetNodes()[0]

	// update the current state
	apiEndpointNode.NodeType = pb.NodeType_master
	newEndpointNode.NodeType = pb.NodeType_apiEndpoint

	if err := utils.ChangeAPIEndpoint(currentK8sClusterInfo.Name, apiEndpointNode.GetPublic(), newEndpointNode.GetPublic(), outputDirectory); err != nil {
		return err
	}

	// Cleanup
	return os.RemoveAll(outputDirectory)
}
