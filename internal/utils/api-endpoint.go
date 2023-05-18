package utils

import (
	"fmt"
	"github.com/berops/claudie/proto/pb"
)

// FindLbAPIEndpoint searches for a role with ApiEndpoint among the LBcluster.
func FindLbAPIEndpoint(lbs []*pb.LBcluster) bool {
	for _, lb := range lbs {
		if HasAPIServerRole(lb.GetRoles()) {
			return true
		}
	}
	return false
}

// HasAPIServerRole checks if there is an API server role.
func HasAPIServerRole(roles []*pb.Role) bool {
	for _, role := range roles {
		if role.RoleType == pb.RoleType_ApiServer {
			return true
		}
	}

	return false
}

// FindAPIEndpointNodePoolWithNode searches for a nodepool that has a node with type ApiEndpoint.
func FindAPIEndpointNodePoolWithNode(nodepools []*pb.NodePool) (*pb.NodePool, *pb.Node, error) {
	for _, nodepool := range nodepools {
		if nodepool.IsControl {
			if node, err := FindEndpointNode(nodepool); err == nil {
				return nodepool, node, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("no node found with type: %s", pb.NodeType_apiEndpoint.String())
}

// FindEndpointNode searches the nodes of the nodepool for a node with type ApiEndpoint.
func FindEndpointNode(nodepool *pb.NodePool) (*pb.Node, error) {
	for _, node := range nodepool.GetNodes() {
		if node.GetNodeType() == pb.NodeType_apiEndpoint {
			return node, nil
		}
	}
	return nil, fmt.Errorf("failed to find node with type %s", pb.NodeType_apiEndpoint.String())
}

// FindControlNode search the nodepools for a node with type Master.
func FindControlNode(nodepools []*pb.NodePool) (*pb.Node, error) {
	for _, nodepool := range nodepools {
		for _, node := range nodepool.Nodes {
			if node.NodeType == pb.NodeType_master {
				return node, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to find node with type %s", pb.NodeType_master.String())
}

// FindAPIEndpointNode searches the NodePools for a Node with type ApiEndpoint.
func FindAPIEndpointNode(nodepools []*pb.NodePool) (*pb.Node, error) {
	for _, nodePool := range nodepools {
		if nodePool.IsControl {
			if node, err := FindEndpointNode(nodePool); err == nil {
				return node, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to find node with type %s", pb.NodeType_apiEndpoint.String())
}
