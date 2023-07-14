package utils

import (
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	cbox "github.com/berops/claudie/services/context-box/client"
)

// SaveConfigWithWorkflowError saves config with workflow states
func SaveConfigWithWorkflowError(config *pb.Config, c pb.ContextBoxServiceClient, clusterView *utils.ClusterView) error {
	config.State = clusterView.ClusterWorkflows
	return cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
}

// updateNodePoolInfo updates the nodepool metadata and node private IPs between stages of the cluster build.
func UpdateNodePoolInfo(src []*pb.NodePool, dst []*pb.NodePool) {
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
					npDst.GetDynamicNodePool().Metadata = npSrc.GetDynamicNodePool().Metadata
				}
				continue src
			}
		}
	}
}

// getNodeMap returns a map of nodes, where each key is node name.
func getNodeMap(nodes []*pb.Node) map[string]*pb.Node {
	m := make(map[string]*pb.Node, len(nodes))
	for _, node := range nodes {
		m[node.Name] = node
	}
	return m
}
