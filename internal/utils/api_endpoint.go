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

// FindNodepoolWithApiEndpointNode searches for a nodepool that has the control node representing
// the Api endpoint of the cluster.
// Returns the control node if found and its corresponding nodepool.
func FindNodepoolWithApiEndpointNode(nodepools []*pb.NodePool) (*pb.NodePool, *pb.Node, error) {
	for _, np := range nodepools {
		if np.IsControl {
			if node, err := FindEndpointNode(np); err == nil {
				return np, node, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("no node found with type: %s", pb.NodeType_apiEndpoint.String())
}

// FindEndpointNode searches the nodes of the nodepool for a node with type ApiEndpoint.
func FindEndpointNode(np *pb.NodePool) (*pb.Node, error) {
	for _, node := range np.GetNodes() {
		if node.GetNodeType() == pb.NodeType_apiEndpoint {
			return node, nil
		}
	}
	return nil, fmt.Errorf("failed to find node with type %s", pb.NodeType_apiEndpoint.String())
}

// FindControlNode search the nodepools for a node with type Master.
func FindControlNode(nodepools []*pb.NodePool) (*pb.Node, error) {
	for _, np := range nodepools {
		for _, node := range np.GetNodes() {
			if node.NodeType == pb.NodeType_master {
				return node, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to find node with type %s", pb.NodeType_master.String())
}

// FindAPIEndpointNode searches the NodePools for a Node with type ApiEndpoint.
func FindAPIEndpointNode(nodepools []*pb.NodePool) (*pb.Node, error) {
	for _, np := range nodepools {
		if np.IsControl {
			if node, err := FindEndpointNode(np); err == nil {
				return node, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to find node with type %s", pb.NodeType_apiEndpoint.String())
}
