package usecases

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/rs/zerolog/log"

	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
)

func (u *Usecases) UpdateAPIEndpoint(request *pb.UpdateAPIEndpointRequest) (*pb.UpdateAPIEndpointResponse, error) {
	if request.Current == nil {
		return &pb.UpdateAPIEndpointResponse{Current: request.Current, Desired: request.Desired}, nil
	}

	log.Info().Msgf("Updating api endpoint for cluster %s project %s", request.Current.ClusterInfo.Name, request.ProjectName)
	if err := updateAPIEndpoint(request.Current.ClusterInfo, request.Desired.ClusterInfo, u.SpawnProcessLimit); err != nil {
		return nil, fmt.Errorf("failed to update api endpoint for cluster %s project %s", request.Current.ClusterInfo.Name, request.ProjectName)
	}
	log.Info().Msgf("Updated api endpoint for cluster %s project %s", request.Current.ClusterInfo.Name, request.ProjectName)

	return &pb.UpdateAPIEndpointResponse{Current: request.Current, Desired: request.Desired}, nil
}

// updateAPIEndpoint handles the case where the ApiEndpoint node is removed from
// the desired state. Thus a new control node needs to be selected among the existing
// control nodes. This new control node will then represent the ApiEndpoint of the cluster.
func updateAPIEndpoint(currentK8sClusterInfo, desiredK8sClusterInfo *pb.ClusterInfo, spawnProcessLimit chan struct{}) error {
	clusterID := commonUtils.GetClusterID(currentK8sClusterInfo)

	apiEndpointNodePool, apiEndpointNode, err := commonUtils.FindNodepoolWithApiEndpointNode(currentK8sClusterInfo.GetNodePools())
	if err != nil {
		return fmt.Errorf("failed to find the node with type: %s", pb.NodeType_apiEndpoint.String())
	}

	apiEndpointNodeExists := slices.ContainsFunc(desiredK8sClusterInfo.GetNodePools(), func(pool *pb.NodePool) bool {
		return pool.GetName() == apiEndpointNodePool.GetName()
	})

	if apiEndpointNodeExists {
		return nil
	}

	// This is the directory where files (Ansible inventory files, SSH keys etc.) will be generated.
	clusterDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, commonUtils.CreateHash(commonUtils.HashLength)))
	if err := commonUtils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	if err := commonUtils.CreateKeysForDynamicNodePools(commonUtils.GetCommonDynamicNodePools(currentK8sClusterInfo.NodePools), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := commonUtils.CreateKeysForStaticNodepools(commonUtils.GetCommonStaticNodePools(currentK8sClusterInfo.NodePools), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	err = utils.GenerateInventoryFile(templates.LoadbalancerInventoryTemplate, clusterDirectory, utils.LBInventoryFileParameters{
		K8sNodepools: utils.NodePools{
			Dynamic: commonUtils.GetCommonDynamicNodePools(currentK8sClusterInfo.NodePools),
			Static:  commonUtils.GetCommonStaticNodePools(currentK8sClusterInfo.NodePools),
		},
		LBClusters: nil,
		ClusterID:  clusterID,
	})
	if err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", clusterDirectory, err)
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

	if err = utils.ChangeAPIEndpoint(currentK8sClusterInfo.Name, apiEndpointNode.GetPublic(), newEndpointNode.GetPublic(), clusterDirectory, spawnProcessLimit); err != nil {
		return err
	}

	return os.RemoveAll(clusterDirectory)
}
