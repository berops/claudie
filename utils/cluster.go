package utils

import (
	"github.com/Berops/platform/proto/pb"
)

// GetClusterByName will return Cluster that will have same name as specified in parameters
// If no name is found, return nil
func GetClusterByName(clusterName string, clusters []*pb.Cluster) *pb.Cluster {
	if clusterName == "" {
		return nil
	}

	if len(clusters) == 0 {
		return nil
	}

	for _, cluster := range clusters {
		if cluster.Name == clusterName {
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
