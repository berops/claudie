package utils

import (
	"fmt"

	"github.com/berops/claudie/proto/pb"
)

// ProviderNames struct hold pair of cloud provider name and user defined name from manifest.
type ProviderNames struct {
	SpecName          string
	CloudProviderName string
}

// GetClusterByName will return index of Cluster that will have same name as specified in parameters
// If no name is found, return -1
func GetClusterByName(clusterName string, clusters []*pb.K8Scluster) int {
	if clusterName == "" {
		return -1
	}

	if len(clusters) == 0 {
		return -1
	}

	for i, cluster := range clusters {
		if cluster == nil {
			continue
		}
		if cluster.ClusterInfo.Name == clusterName {
			return i
		}
	}

	return -1
}

// GetLBClusterByName will return index of Cluster that will have same name as specified in parameters
// If no name is found, return -1
func GetLBClusterByName(name string, clusters []*pb.LBcluster) int {
	if name == "" {
		return -1
	}

	if len(clusters) == 0 {
		return -1
	}

	for i, cluster := range clusters {
		if cluster.ClusterInfo.Name == name {
			return i
		}
	}

	return -1
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
func GetRegions(nodepools []*pb.DynamicNodePool) []string {
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

// GroupNodepoolsByTemplates groups nodepool by template repository
func GroupNodepoolsByTemplates(nodepools []*pb.NodePool) map[string][]*pb.NodePool {
	result := make(map[string][]*pb.NodePool)
	for _, np := range nodepools {
		key := fmt.Sprintf("%s-%s-%s",
			np.GetDynamicNodePool().GetTemplates().GetRepository(),
			np.GetDynamicNodePool().GetTemplates().GetTag(),
			np.GetDynamicNodePool().GetTemplates().GetPath(),
		)
		result[key] = append(result[key], np)
	}
	return result
}

// GroupNodepoolsByProviderNames groups nodepool by provider spec name into the map[Provider Names][]*pb.Nodepool
func GroupNodepoolsByProviderNames(clusterInfo *pb.ClusterInfo) map[ProviderNames][]*pb.NodePool {
	sortedNodePools := map[ProviderNames][]*pb.NodePool{}
	pnStatic := ProviderNames{SpecName: pb.StaticNodepoolInfo_STATIC_PROVIDER.String(), CloudProviderName: pb.StaticNodepoolInfo_STATIC_PROVIDER.String()}
	for _, nodepool := range clusterInfo.GetNodePools() {
		if np := nodepool.GetDynamicNodePool(); np != nil {
			pn := ProviderNames{SpecName: np.Provider.SpecName, CloudProviderName: np.Provider.CloudProviderName}
			sortedNodePools[pn] = append(sortedNodePools[pn], nodepool)
		} else if np := nodepool.GetStaticNodePool(); np != nil {
			sortedNodePools[pnStatic] = append(sortedNodePools[pnStatic], nodepool)
		}
	}
	return sortedNodePools
}

// GroupNodepoolsByProviderSpecName groups nodepool by provider spec name into the map[Provider Name][]*pb.Nodepool
func GroupNodepoolsByProviderSpecName(clusterInfo *pb.ClusterInfo) map[string][]*pb.NodePool {
	sortedNodePools := map[string][]*pb.NodePool{}
	for _, nodepool := range clusterInfo.GetNodePools() {
		if np := nodepool.GetDynamicNodePool(); np != nil {
			sortedNodePools[np.Provider.SpecName] = append(sortedNodePools[np.Provider.SpecName], nodepool)
		} else if np := nodepool.GetStaticNodePool(); np != nil {
			sortedNodePools[pb.StaticNodepoolInfo_STATIC_PROVIDER.String()] = append(sortedNodePools[pb.StaticNodepoolInfo_STATIC_PROVIDER.String()], nodepool)
		}
	}
	return sortedNodePools
}

// GroupNodepoolsByProviderRegion groups nodepool by cloud provider instance name and region into the map[<provider-instance-name>-<region>][]*pb.Nodepool
func GroupNodepoolsByProviderRegion(clusterInfo *pb.ClusterInfo) map[string][]*pb.NodePool {
	sortedNodePools := map[string][]*pb.NodePool{}
	for _, nodepool := range clusterInfo.GetNodePools() {
		if np := nodepool.GetDynamicNodePool(); np != nil {
			key := fmt.Sprintf("%s-%s", np.Provider.SpecName, np.Region)
			sortedNodePools[key] = append(sortedNodePools[key], nodepool)
		} else if np := nodepool.GetStaticNodePool(); np != nil {
			key := fmt.Sprintf("%s-%s", pb.StaticNodepoolInfo_STATIC_PROVIDER.String(), pb.StaticNodepoolInfo_STATIC_REGION.String())
			sortedNodePools[key] = append(sortedNodePools[key], nodepool)
		}
	}
	return sortedNodePools
}

// FindName will return a real node name based on the user defined one
// example: name defined in cloud provider: gcp-control-1 -> name defined in cluster : gcp-control-1
func FindName(realNames []string, name string) string {
	for _, n := range realNames {
		if name == n {
			return n
		}
	}
	return ""
}

// IsAutoscaled returns true, if cluster has at least one nodepool with autoscaler config.
func IsAutoscaled(cluster *pb.K8Scluster) bool {
	if cluster == nil {
		return false
	}
	for _, np := range cluster.ClusterInfo.NodePools {
		if n := np.GetDynamicNodePool(); n != nil {
			if n.AutoscalerConfig != nil {
				return true
			}
		}
	}
	return false
}

// GetDynamicNodePoolsFromCI returns slice of dynamic node pools used in specified cluster info.
func GetDynamicNodePoolsFromCI(ci *pb.ClusterInfo) []*pb.DynamicNodePool {
	if ci == nil {
		return nil
	}

	nps := make([]*pb.DynamicNodePool, 0, len(ci.GetNodePools()))
	for _, np := range ci.GetNodePools() {
		if n := np.GetDynamicNodePool(); n != nil {
			nps = append(nps, n)
		}
	}
	return nps
}

// GetDynamicNodePools returns slice of dynamic node pools.
func GetDynamicNodePools(nps []*pb.NodePool) []*pb.DynamicNodePool {
	dnps := make([]*pb.DynamicNodePool, 0, len(nps))
	for _, np := range nps {
		if n := np.GetDynamicNodePool(); n != nil {
			dnps = append(dnps, n)
		}
	}
	return dnps
}

// GetCommonStaticNodePools returns slice of common node pools, where every node pool is static.
func GetCommonStaticNodePools(nps []*pb.NodePool) []*pb.NodePool {
	static := make([]*pb.NodePool, 0, len(nps))
	for _, n := range nps {
		if n.GetStaticNodePool() != nil {
			static = append(static, n)
		}
	}
	return static
}

// GetCommonDynamicNodePools returns slice of common node pools, where every node pool is dynamic.
func GetCommonDynamicNodePools(nps []*pb.NodePool) []*pb.NodePool {
	dynamic := make([]*pb.NodePool, 0, len(nps))
	for _, n := range nps {
		if n.GetDynamicNodePool() != nil {
			dynamic = append(dynamic, n)
		}
	}
	return dynamic
}

func CountLbNodes(lb *pb.LBcluster) int {
	var out int
	for _, np := range lb.GetClusterInfo().GetNodePools() {
		switch i := np.GetNodePoolType().(type) {
		case *pb.NodePool_DynamicNodePool:
			out += int(i.DynamicNodePool.Count)
		case *pb.NodePool_StaticNodePool:
			// Lbs are only dynamic.
		}
	}

	return out
}

func CountNodes(k *pb.K8Scluster) int {
	var out int
	for _, np := range k.GetClusterInfo().GetNodePools() {
		switch i := np.GetNodePoolType().(type) {
		case *pb.NodePool_DynamicNodePool:
			out += int(i.DynamicNodePool.Count)
		case *pb.NodePool_StaticNodePool:
			out += len(i.StaticNodePool.GetNodeKeys())
		}
	}
	return out
}
