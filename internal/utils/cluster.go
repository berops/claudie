package utils

import (
	"strings"

	"github.com/Berops/claudie/proto/pb"
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

// GetRegions will return a list of all regions used in list of nodepools
func GetRegions(nodepools []*pb.NodePool) []string {
	// create a set of region
	regionSet := make(map[string]struct{})
	for _, nodepool := range nodepools {
		regionSet[nodepool.Region] = struct{}{}
	}

	// extract value of set and create a slice
	var regions []string
	for k := range regionSet {
		regions = append(regions, k)
	}
	return regions
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
// example: name defined in cloud provider: gcp-cluster-jkshbdc-gcp-control-1 -> name defined in cluster : gcp-control-1
func FindName(realNames []string, name string) string {
	for _, n := range realNames {
		if strings.Contains(name, n) {
			return n
		}
	}
	return ""
}
