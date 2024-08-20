package utils

import (
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog/log"
)

//// SaveConfigWithWorkflowError saves config with workflow states
//func SaveConfigWithWorkflowError(config *spec.Config, c pb.ContextBoxServiceClient, clusterView *utils.ClusterView) error {
//	config.State = clusterView.ClusterWorkflows
//	return cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
//}

// updateNodePoolInfo updates the nodepool CIDR and node private IPs between stages of the cluster build.
func UpdateNodePoolInfo(src []*spec.NodePool, dst []*spec.NodePool) {
src:
	for _, npSrc := range src {
		for _, npDst := range dst {
			if npSrc.Name == npDst.Name {
				srcNodes := getNodeMap(npSrc.Nodes)
				dstNodes := getNodeMap(npDst.Nodes)
				for dstName, dstNode := range dstNodes {
					if srcNode, ok := srcNodes[dstName]; ok {
						dstNode.Private = srcNode.Private
					}
				}
				if npSrc.GetDynamicNodePool() != nil && npDst.GetDynamicNodePool() != nil {
					npDst.GetDynamicNodePool().Cidr = npSrc.GetDynamicNodePool().Cidr
				}
				continue src
			}
		}
	}
}

// getNodeMap returns a map of nodes, where each key is node name.
func getNodeMap(nodes []*spec.Node) map[string]*spec.Node {
	m := make(map[string]*spec.Node, len(nodes))
	for _, node := range nodes {
		m[node.Name] = node
	}
	return m
}

// separateNodepools creates two slices of node names, one for master and one for worker nodes
func SeparateNodepools(clusterNodes map[string]int32, currentClusterInfo, desiredClusterInfo *spec.ClusterInfo) (master []string, worker []string) {
	for _, nodepool := range currentClusterInfo.NodePools {
		var names = make([]string, 0, len(nodepool.Nodes))
		if np := nodepool.GetDynamicNodePool(); np != nil {
			if count, ok := clusterNodes[nodepool.Name]; ok && count > 0 {
				names = getDynamicNodeNames(nodepool, int(count))
			}
		} else if np := nodepool.GetStaticNodePool(); np != nil {
			if count, ok := clusterNodes[nodepool.Name]; ok && count > 0 {
				names = getStaticNodeNames(nodepool, desiredClusterInfo)
			}
		}
		if nodepool.IsControl {
			master = append(master, names...)
		} else {
			worker = append(worker, names...)
		}
	}
	return master, worker
}

// getDynamicNodeNames returns slice of length count with names of the nodes from specified nodepool
// nodes chosen are from the last element in Nodes slice, up to the first one
func getDynamicNodeNames(np *spec.NodePool, count int) (names []string) {
	for i := len(np.GetNodes()) - 1; i >= len(np.GetNodes())-count; i-- {
		names = append(names, np.GetNodes()[i].GetName())
		log.Debug().Msgf("Choosing node %s for deletion", np.GetNodes()[i].GetName())
	}
	return names
}

// getStaticNodeNames returns slice of length count with names of the nodes from specified nodepool
// nodes chosen are from the last element in Nodes slice, up to the first one
func getStaticNodeNames(np *spec.NodePool, desiredCluster *spec.ClusterInfo) (names []string) {
	// Find desired nodes for node pool.
	desired := make(map[string]struct{})
	for _, n := range desiredCluster.NodePools {
		if n.Name == np.Name {
			for _, node := range n.Nodes {
				desired[node.Name] = struct{}{}
			}
		}
	}
	// Find deleted nodes
	if n := np.GetStaticNodePool(); n != nil {
		for _, node := range np.Nodes {
			if _, ok := desired[node.Name]; !ok {
				// Append name as it is not defined in desired state.
				names = append(names, node.Name)
			}
		}
	}
	return names
}
