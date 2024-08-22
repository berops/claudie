package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

const (
	defaulHttpProxyMode = "default"
)

func (u *Usecases) UpdateNoProxyEnvs(request *pb.UpdateNoProxyEnvsRequest) (*pb.UpdateNoProxyEnvsResponse, error) {
	if request.Current == nil {
		return &pb.UpdateNoProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
	}

	hasHetznerNodeFlag := hasHetznerNode(request.Desired.ClusterInfo)
	httpProxyMode := commonUtils.GetEnvDefault("HTTP_PROXY_MODE", defaulHttpProxyMode)
	// Changing NO_PROXY and no_proxy env variables is necessary only when
	// 1. HTTP Proxy mode isn't "off"
	// 2. cluster has a Hetzner node or cluster is being build using HTTP proxy
	if !(httpProxyMode == "on" || (httpProxyMode != "off" && hasHetznerNodeFlag)) {
		return &pb.UpdateNoProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
	}

	nodesChangedFlag := nodesChanged(request.Current.ClusterInfo, request.Desired.ClusterInfo)
	// At least one node in the cluster has to be changed to perform the update of NO_PROXY and no_proxy env variables.
	if !nodesChangedFlag {
		return &pb.UpdateNoProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
	}

	log.Info().Msgf("Updating NO_PROXY and no_proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
		request.Current.ClusterInfo.Name, request.ProjectName)
	if err := updateNoProxyEnvs(request.Current.ClusterInfo, request.Desired.ClusterInfo, u.SpawnProcessLimit); err != nil {
		return nil, fmt.Errorf("Failed to update NO_PROXY and no_proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
			request.Current.ClusterInfo.Name, request.ProjectName)
	}
	log.Info().Msgf("Updated NO_PROXY and no_proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
		request.Current.ClusterInfo.Name, request.ProjectName)

	return &pb.UpdateNoProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
}

func hasHetznerNode(desiredK8sClusterInfo *pb.ClusterInfo) bool {
	desiredNodePools := desiredK8sClusterInfo.GetNodePools()
	for _, np := range desiredNodePools {
		if np.GetDynamicNodePool() != nil && np.GetDynamicNodePool().Provider.CloudProviderName == "hetzner" {
			return true
		}
	}

	return false
}

func nodesChanged(currentK8sClusterInfo, desiredK8sClusterInfo *pb.ClusterInfo) bool {
	currNodePool := currentK8sClusterInfo.GetNodePools()
	desiredNodePool := desiredK8sClusterInfo.GetNodePools()

	currStateNodesNum := 0
	currStateIpMap := make(map[string]bool)
	for _, np := range currNodePool {
		for _, node := range np.Nodes {
			currStateIpMap[node.Public] = true
			currStateIpMap[node.Private] = true
			currStateNodesNum += 1
		}
	}

	desiredStateNodesNum := 0
	for _, np := range desiredNodePool {
		for _, node := range np.Nodes {
			desiredStateNodesNum += 1
			_, publicIPExists := currStateIpMap[node.Public]
			_, privateIPExists := currStateIpMap[node.Private]

			// Node in at least one nodepool was changed and no proxy env variables have to be changed.
			if !publicIPExists || !privateIPExists {
				return true
			}
		}
	}

	// Returns true if at lease one node was added or removed from the cluster.
	// Otherwise returns false.
	return desiredStateNodesNum != currStateNodesNum
}

// updateNoProxyEnvs handles the case where Claudie adds/removes node to/from the cluster.
// Public and private IPs of this node must be added to the NO_PROXY and no_proxy env variables in
// kube-proxy DaemonSet and static pods.
func updateNoProxyEnvs(currentK8sClusterInfo, desiredK8sClusterInfo *pb.ClusterInfo, spawnProcessLimit chan struct{}) error {
	return nil
}
