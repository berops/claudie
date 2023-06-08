package utils

import (
	"fmt"
	"strings"

	"github.com/berops/claudie/proto/pb"
)

// ProviderNames struct hold pair of cloud provider name and user defined name from manifest.
type ProviderNames struct {
	SpecName          string
	CloudProviderName string
}

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

// GroupNodepoolsByProviderNames groups nodepool by provider spec name into the map[Provider Names][]*pb.Nodepool
func GroupNodepoolsByProviderNames(clusterInfo *pb.ClusterInfo) map[ProviderNames][]*pb.NodePool {
	sortedNodePools := map[ProviderNames][]*pb.NodePool{}
	pnStatic := ProviderNames{SpecName: pb.StaticProvider_STATIC_PROVIDER.String(), CloudProviderName: pb.StaticProvider_STATIC_PROVIDER.String()}
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
			sortedNodePools[pb.StaticProvider_STATIC_PROVIDER.String()] = append(sortedNodePools[pb.StaticProvider_STATIC_PROVIDER.String()], nodepool)
		}
	}
	return sortedNodePools
}

// GroupNodepoolsByProvider groups nodepool by cloud provider name into the map[Provider Name][]*pb.Nodepool
func GroupNodepoolsByProvider(clusterInfo *pb.ClusterInfo) map[string][]*pb.NodePool {
	sortedNodePools := map[string][]*pb.NodePool{}
	for _, nodepool := range clusterInfo.GetNodePools() {
		if np := nodepool.GetDynamicNodePool(); np != nil {
			sortedNodePools[np.Provider.CloudProviderName] = append(sortedNodePools[np.Provider.CloudProviderName], nodepool)
		} else if np := nodepool.GetStaticNodePool(); np != nil {
			sortedNodePools[pb.StaticProvider_STATIC_PROVIDER.String()] = append(sortedNodePools[pb.StaticProvider_STATIC_PROVIDER.String()], nodepool)
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
			key := fmt.Sprintf("%s-%s", pb.StaticProvider_STATIC_PROVIDER.String(), pb.StaticRegion_STATIC_REGION.String())
			sortedNodePools[key] = append(sortedNodePools[key], nodepool)
		}
	}
	return sortedNodePools
}

// FindName will return a real node name based on the user defined one
// example: name defined in cloud provider: gcp-cluster-jkshbdc-gcp-control-1 -> name defined in cluster : gcp-control-1
func FindName(realNames []string, name string) string {
	for _, n := range realNames {
		if strings.Contains(name, n) {
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
		if n := np.GetDynamicNodePool(); np != nil {
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

// GetStaticNodePools returns slice of dynamic node pools.
func GetStaticNodePools(nps []*pb.NodePool) []*pb.StaticNodePool {
	snps := make([]*pb.StaticNodePool, 0, len(nps))
	for _, np := range nps {
		if n := np.GetStaticNodePool(); n != nil {
			snps = append(snps, n)
		}
	}
	return snps
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
