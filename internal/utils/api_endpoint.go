package utils

import (
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
)

// HasAPIServerRole checks if there is an API server role.
func HasAPIServerRole(roles []*spec.Role) bool {
	for _, role := range roles {
		if role.RoleType == spec.RoleType_ApiServer {
			return true
		}
	}

	return false
}

// FindNodepoolWithApiEndpointNode searches for a nodepool that has the control node representing
// the Api endpoint of the cluster.
// Returns the control node if found and its corresponding nodepool.
func FindNodepoolWithApiEndpointNode(nodepools []*spec.NodePool) (*spec.NodePool, *spec.Node, error) {
	for _, np := range nodepools {
		if np.IsControl {
			if node, err := FindEndpointNode(np); err == nil {
				return np, node, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("no node found with type: %s", spec.NodeType_apiEndpoint.String())
}

// FindEndpointNode searches the nodes of the nodepool for a node with type ApiEndpoint.
func FindEndpointNode(np *spec.NodePool) (*spec.Node, error) {
	for _, node := range np.GetNodes() {
		if node.GetNodeType() == spec.NodeType_apiEndpoint {
			return node, nil
		}
	}
	return nil, fmt.Errorf("failed to find node with type %s", spec.NodeType_apiEndpoint.String())
}

// FindControlNode search the nodepools for a node with type Master.
func FindControlNode(nodepools []*spec.NodePool) (*spec.Node, error) {
	for _, np := range nodepools {
		for _, node := range np.GetNodes() {
			if node.NodeType == spec.NodeType_master {
				return node, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to find node with type %s", spec.NodeType_master.String())
}

// FindControlNodepools returns control nodepools
func FindControlNodepools(nodepools []*spec.NodePool) []*spec.NodePool {
	var result []*spec.NodePool

	for _, np := range nodepools {
		if np.IsControl {
			result = append(result, np)
		}
	}

	return result
}

// FindAPIEndpointNode searches the NodePools for a Node with type ApiEndpoint.
func FindAPIEndpointNode(nodepools []*spec.NodePool) (*spec.Node, error) {
	for _, np := range nodepools {
		if np.IsControl {
			if node, err := FindEndpointNode(np); err == nil {
				return node, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to find node with type %s", spec.NodeType_apiEndpoint.String())
}
