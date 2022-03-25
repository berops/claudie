package utils

import (
	"github.com/Berops/platform/proto/pb"
)

// GetClusterByName will return Cluster that will have same name as specified in parameters
// If no name is found, return nil
func GetClusterByName(clusterName string, clusters []*pb.K8Scluster) *pb.K8Scluster {
	if clusterName == "" {
		return nil
	}

	if len(clusters) == 0 {
		return nil
	}

	for _, cluster := range clusters {
		if cluster.ClusterInfo.Name == clusterName {
			return cluster
		}
	}

	return nil
}

// GetNodePoolByName will return first Nodepool that will have same name as specified in parameters
// If no name is found, return nil
func GetNodePoolByName(nodePoolName string, nodePools []*pb.NodePool) *pb.NodePool {
	if nodePoolName == "" {
		return nil
	}
	for _, np := range nodePools {
		if np.Name == nodePoolName {
			return np
		}
	}
	return nil
}

// groups nodepool by provider name into the map[Provider Name][]*pb.Nodepool
func GroupNodepoolsByProvider(clusterInfo *pb.ClusterInfo) map[string][]*pb.NodePool {
	sortedNodePools := map[string][]*pb.NodePool{}
	for _, nodepool := range clusterInfo.GetNodePools() {
		sortedNodePools[nodepool.Provider.Name] = append(sortedNodePools[nodepool.Provider.Name], nodepool)
	}
	return sortedNodePools
}
