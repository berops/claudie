package utils

import (
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
)

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

func GetCommonDynamicControlPlaneNodes(currentNp, desiredNp []*spec.NodePool) []*spec.NodePool {
	currDynamicControlNps := make(map[string]*spec.NodePool)
	for _, np := range currentNp {
		if np.IsControl && np.GetDynamicNodePool() != nil {
			currDynamicControlNps[np.Name] = np
		}
	}

	commonDynamicControlNps := CreateNpsFromCommonControlPlaneNodes(currDynamicControlNps, desiredNp)
	return commonDynamicControlNps
}

func GetCommonStaticControlPlaneNodes(currentNp, desiredNp []*spec.NodePool) []*spec.NodePool {
	currStaticControlNps := make(map[string]*spec.NodePool)
	for _, np := range currentNp {
		if np.IsControl && np.GetStaticNodePool() != nil {
			currStaticControlNps[np.Name] = np
		}
	}

	commonStaticControlNps := CreateNpsFromCommonControlPlaneNodes(currStaticControlNps, desiredNp)
	return commonStaticControlNps
}

func CreateNpsFromCommonControlPlaneNodes(currControlNps map[string]*spec.NodePool, desiredNp []*spec.NodePool) []*spec.NodePool {
	var commonControlNps []*spec.NodePool

	for _, np := range desiredNp {
		if currNp, exists := currControlNps[np.Name]; exists {
			currNodeMap := make(map[string]*spec.Node)
			for _, node := range currNp.Nodes {
				currNodeMap[node.Name] = node
			}
			var commonNodes []*spec.Node
			for _, node := range np.Nodes {
				if _, exists := currNodeMap[node.Name]; exists {
					commonNodes = append(commonNodes, node)
				}
			}

			if len(commonNodes) > 0 {
				// copy everything except Nodes
				commonNodePool := &spec.NodePool{
					Type:        currNp.Type,
					Name:        currNp.Name,
					Nodes:       commonNodes,
					IsControl:   currNp.IsControl,
					Labels:      currNp.Labels,
					Annotations: currNp.Annotations,
				}
				commonControlNps = append(commonControlNps, commonNodePool)
			}
		}
	}

	return commonControlNps
}

func CountLbNodes(lb *spec.LBcluster) int {
	var out int
	for _, np := range lb.GetClusterInfo().GetNodePools() {
		switch i := np.Type.(type) {
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
		switch i := np.Type.(type) {
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
