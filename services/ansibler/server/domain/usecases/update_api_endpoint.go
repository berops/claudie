package usecases

import (
	"fmt"
	"os"
	"path/filepath"

	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog/log"
)

const (
	defaultHttpProxyMode = "default"
	defaultHttpProxyUrl  = "http://proxy.claudie.io:8880"
	noProxyDefault       = "127.0.0.1/8,localhost,cluster.local,10.244.0.0/16,10.96.0.0/12" // 10.244.0.0/16 is kubeone's default PodCIDR and 10.96.0.0/12 is kubeone's default ServiceCIDR
)

func (u *Usecases) UpdateAPIEndpoint(request *pb.UpdateAPIEndpointRequest) (*pb.UpdateAPIEndpointResponse, error) {
	if request.Current == nil {
		return &pb.UpdateAPIEndpointResponse{Current: request.Current}, nil
	}

	log.Info().Msgf("Updating api endpoint for cluster %s project %s", request.Current.ClusterInfo.Name, request.ProjectName)
	if err := updateAPIEndpoint(request.Endpoint, request.Current.ClusterInfo, request.Desired.ClusterInfo, request.DesiredLbs, u.SpawnProcessLimit); err != nil {
		return nil, fmt.Errorf("failed to update api endpoint for cluster %s project %s", request.Current.ClusterInfo.Name, request.ProjectName)
	}
	log.Info().Msgf("Updated api endpoint for cluster %s project %s", request.Current.ClusterInfo.Name, request.ProjectName)

	return &pb.UpdateAPIEndpointResponse{Current: request.Current, Desired: request.Desired}, nil
}

// updateAPIEndpoint handles the case where the ApiEndpoint node is removed from
// the desired state. Thus, a new control node needs to be selected among the existing
// control nodes. This new control node will then represent the ApiEndpoint of the cluster.
func updateAPIEndpoint(endpoint *pb.UpdateAPIEndpointRequest_Endpoint, currentK8sClusterInfo, desiredK8sClusterInfo *spec.ClusterInfo, desiredLbs []*spec.LBcluster, spawnProcessLimit chan struct{}) error {
	clusterID := commonUtils.GetClusterID(currentK8sClusterInfo)

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

	// TODO: make sure new endpoint is in the no proxy list
	httpProxyUrl, noProxyList := utils.GetHttpProxyUrlAndNoProxyList(desiredK8sClusterInfo, desiredLbs)

	err := utils.GenerateInventoryFile(templates.LoadbalancerInventoryTemplate, clusterDirectory, utils.LBInventoryFileParameters{
		K8sNodepools: utils.NodePools{
			Dynamic: commonUtils.GetCommonDynamicNodePools(currentK8sClusterInfo.NodePools),
			Static:  commonUtils.GetCommonStaticNodePools(currentK8sClusterInfo.NodePools),
		},
		LBClusters:   nil,
		ClusterID:    clusterID,
		NoProxyList:  noProxyList,
		HttpProxyUrl: httpProxyUrl,
	})
	if err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", clusterDirectory, err)
	}

	_, apiEndpointNode, err := commonUtils.FindNodepoolWithApiEndpointNode(currentK8sClusterInfo.NodePools)
	if err != nil {
		return fmt.Errorf("current state cluster doesn't have api endpoint as a control plane node")
	}

	np := commonUtils.GetNodePoolByName(endpoint.Nodepool, currentK8sClusterInfo.NodePools)
	if np == nil {
		return fmt.Errorf("no nodepool %q found within current state", endpoint.Nodepool)
	}

	var newEndpointNode *spec.Node

	for _, node := range np.Nodes {
		if node.Name == endpoint.Node {
			newEndpointNode = node
			break
		}
	}

	if newEndpointNode == nil {
		return fmt.Errorf("no node %q within nodepool %q found in current state", endpoint.Node, endpoint.Nodepool)
	}

	// update the current state
	apiEndpointNode.NodeType = spec.NodeType_master
	newEndpointNode.NodeType = spec.NodeType_apiEndpoint

	if err = utils.ChangeAPIEndpoint(currentK8sClusterInfo.Name, apiEndpointNode.GetPublic(), newEndpointNode.GetPublic(), clusterDirectory, spawnProcessLimit); err != nil {
		return err
	}

	return os.RemoveAll(clusterDirectory)
}
