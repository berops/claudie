package utils

import (
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
)

// ProviderNames struct hold pair of cloud provider name and user defined name from manifest.
type ProviderNames struct {
	SpecName          string
	CloudProviderName string
}

// GetLBClusterByName will return index of Cluster that will have same name as specified in parameters
// If no name is found, return -1
func GetLBClusterByName(name string, clusters []*spec.LBcluster) int {
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
func GetNodePoolByName(nodePoolName string, nodePools []*spec.NodePool) *spec.NodePool {
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
func GetRegions(nodepools []*spec.DynamicNodePool) []string {
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

// GroupNodepoolsByProviderNames groups nodepool by provider spec name into the map[Provider Names][]*spec.Nodepool
func GroupNodepoolsByProviderNames(clusterInfo *spec.ClusterInfo) map[ProviderNames][]*spec.NodePool {
	sortedNodePools := map[ProviderNames][]*spec.NodePool{}
	pnStatic := ProviderNames{SpecName: spec.StaticNodepoolInfo_STATIC_PROVIDER.String(), CloudProviderName: spec.StaticNodepoolInfo_STATIC_PROVIDER.String()}
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

// GroupNodepoolsByProviderSpecName groups nodepool by provider spec name into the map[Provider Name][]*spec.Nodepool
func GroupNodepoolsByProviderSpecName(clusterInfo *spec.ClusterInfo) map[string][]*spec.NodePool {
	sortedNodePools := map[string][]*spec.NodePool{}
	for _, nodepool := range clusterInfo.GetNodePools() {
		if np := nodepool.GetDynamicNodePool(); np != nil {
			sortedNodePools[np.Provider.SpecName] = append(sortedNodePools[np.Provider.SpecName], nodepool)
		} else if np := nodepool.GetStaticNodePool(); np != nil {
			sortedNodePools[spec.StaticNodepoolInfo_STATIC_PROVIDER.String()] = append(sortedNodePools[spec.StaticNodepoolInfo_STATIC_PROVIDER.String()], nodepool)
		}
	}
	return sortedNodePools
}

// GroupNodepoolsByProviderRegion groups nodepool by cloud provider instance name and region into the map[<provider-instance-name>-<region>][]*pb.Nodepool
func GroupNodepoolsByProviderRegion(clusterInfo *spec.ClusterInfo) map[string][]*spec.NodePool {
	sortedNodePools := map[string][]*spec.NodePool{}
	for _, nodepool := range clusterInfo.GetNodePools() {
		if np := nodepool.GetDynamicNodePool(); np != nil {
			key := fmt.Sprintf("%s-%s", np.Provider.SpecName, np.Region)
			sortedNodePools[key] = append(sortedNodePools[key], nodepool)
		} else if np := nodepool.GetStaticNodePool(); np != nil {
			key := fmt.Sprintf("%s-%s", spec.StaticNodepoolInfo_STATIC_PROVIDER.String(), spec.StaticNodepoolInfo_STATIC_REGION.String())
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
func IsAutoscaled(cluster *spec.K8Scluster) bool {
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
func GetDynamicNodePoolsFromCI(ci *spec.ClusterInfo) []*spec.DynamicNodePool {
	if ci == nil {
		return nil
	}

	nps := make([]*spec.DynamicNodePool, 0, len(ci.GetNodePools()))
	for _, np := range ci.GetNodePools() {
		if n := np.GetDynamicNodePool(); n != nil {
			nps = append(nps, n)
		}
	}
	return nps
}

// GetDynamicNodePools returns slice of dynamic node pools.
func GetDynamicNodePools(nps []*spec.NodePool) []*spec.DynamicNodePool {
	dnps := make([]*spec.DynamicNodePool, 0, len(nps))
	for _, np := range nps {
		if n := np.GetDynamicNodePool(); n != nil {
			dnps = append(dnps, n)
		}
	}
	return dnps
}

// GetCommonStaticNodePools returns slice of common node pools, where every node pool is static.
func GetCommonStaticNodePools(nps []*spec.NodePool) []*spec.NodePool {
	static := make([]*spec.NodePool, 0, len(nps))
	for _, n := range nps {
		if n.GetStaticNodePool() != nil {
			static = append(static, n)
		}
	}
	return static
}

// GetCommonDynamicNodePools returns slice of common node pools, where every node pool is dynamic.
func GetCommonDynamicNodePools(nps []*spec.NodePool) []*spec.NodePool {
	dynamic := make([]*spec.NodePool, 0, len(nps))
	for _, n := range nps {
		if n.GetDynamicNodePool() != nil {
			dynamic = append(dynamic, n)
		}
	}
	return dynamic
}

func CountLbNodes(lb *spec.LBcluster) int {
	var out int
	for _, np := range lb.GetClusterInfo().GetNodePools() {
		switch i := np.GetNodePoolType().(type) {
		case *spec.NodePool_DynamicNodePool:
			out += int(i.DynamicNodePool.Count)
		case *spec.NodePool_StaticNodePool:
			// Lbs are only dynamic.
		}
	}

	return out
}

func CountNodes(k *spec.K8Scluster) int {
	var out int
	for _, np := range k.GetClusterInfo().GetNodePools() {
		switch i := np.GetNodePoolType().(type) {
		case *spec.NodePool_DynamicNodePool:
			out += int(i.DynamicNodePool.Count)
		case *spec.NodePool_StaticNodePool:
			out += len(i.StaticNodePool.GetNodeKeys())
		}
	}
	return out
}

// ExtractTargetPorts extracts target ports defined inside the role in the LoadBalancer.
func ExtractTargetPorts(loadBalancers []*spec.LBcluster) []int {
	ports := make(map[int32]struct{})

	var result []int
	for _, c := range loadBalancers {
		for _, role := range c.Roles {
			if _, ok := ports[role.TargetPort]; !ok {
				result = append(result, int(role.TargetPort))
			}
			ports[role.TargetPort] = struct{}{}
		}
	}

	return result
}
