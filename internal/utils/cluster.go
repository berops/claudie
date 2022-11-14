package utils

import (
	"strings"

	"github.com/Berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
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

// groups nodepool by provider spec name into the map[Provider Name][]*pb.Nodepool
func GroupNodepoolsByProviderSpecName(clusterInfo *pb.ClusterInfo) map[string][]*pb.NodePool {
	sortedNodePools := map[string][]*pb.NodePool{}
	for _, nodepool := range clusterInfo.GetNodePools() {
		sortedNodePools[nodepool.Provider.SpecName] = append(sortedNodePools[nodepool.Provider.SpecName], nodepool)
	}
	return sortedNodePools
}

// groups nodepool by cloud provider name into the map[Provider Name][]*pb.Nodepool
func GroupNodepoolsByProvider(clusterInfo *pb.ClusterInfo) map[string][]*pb.NodePool {
	sortedNodePools := map[string][]*pb.NodePool{}
	for _, nodepool := range clusterInfo.GetNodePools() {
		sortedNodePools[nodepool.Provider.CloudProviderName] = append(sortedNodePools[nodepool.Provider.CloudProviderName], nodepool)
	}
	return sortedNodePools
}

// findName will return a real node name based on the user defined one
// this is needed in case of e.g. GCP, where nodes have some info appended to their which cannot be read from terraform output
// example: gcp-control-1 -> gcp-control-1.c.project.id
func FindName(realNames []string, name string) string {
	for _, n := range realNames {
		if strings.Contains(n, name) {
			return n
		}
	}
	log.Error().Msgf("Error: no real name found for %s", name)
	return ""
}
