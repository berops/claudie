package service

import (
	"fmt"
	"slices"

	"github.com/berops/claudie/proto/pb/spec"
)

type (
	// NodePoolsDiffResult hold all of the changes between two different [NodePoolViewType]
	NodePoolsDiffResult struct {
		// partialDeleted holds which nodepools were partially deleted, meaning that
		// the nodepools is present in both [NodePoolsViewType] but some of the nodes
		// are missing.
		//
		// The data is stored as map[NodePool.Name][]node.Names
		// Each entry in the map contains the NodePool and the Nodes that are
		// present in the first[NodePoolsViewType] but not in the other.
		PartiallyDeleted NodePoolsViewType

		// delete holds which nodepools were completely deleted, meaning that
		// they're present in one [NodePoolsViewType] but no the other.
		//
		// The data is stored as map[NodePool.Name][]node.Names
		// Each entry in the map contains all of the node Names that are
		// present in the first [NodePoolsViewType] but not in the other.
		Deleted NodePoolsViewType

		// Same as [NodePoolsDiffResult.partiallyDeleted] but with newly added nodes.
		PartiallyAdded NodePoolsViewType

		// Same as [NodePoolsDiffResult.deleted] but with newly added Nodepools and their nodes.
		Added NodePoolsViewType
	}

	// NodePoolsViewType is an unordered view into the nodepools and their nodes that are read from a [spec.K8Scluster]
	NodePoolsViewType = map[string][]string
)

func reconciliate(current, desired *spec.Clusters) *spec.TaskEvent {
	if delta := reconciliateK8s(current.K8S, desired.K8S); delta != nil {
		return delta
	}

	if delta := reconciliateLBs(current.LoadBalancers, desired.LoadBalancers); delta != nil {
		return delta
	}

	return nil
}

func reconciliateK8s(current, desired *spec.K8Scluster) *spec.TaskEvent {
	currentDynamic, currentStatic := NodePoolsView(current)
	desiredDynamic, desiredStatic := NodePoolsView(desired)

	// Node names are transferred over from current state based on the public IP.
	// Thus, at this point we can figure out based on nodes names which were deleted/added
	// see existing_state.go:transferStaticNodes
	staticDiff := NodePoolsDiff(currentStatic, desiredStatic)
	dynamicDiff := NodePoolsDiff(currentDynamic, desiredDynamic)

	_ = staticDiff
	_ = dynamicDiff

	return nil
}

func reconciliateLBs(current, desired *spec.LoadBalancers) *spec.TaskEvent {
	return nil
}

// NodePoolsDiff calculates difference between two [NodePoolsViewType] and returns the result as a [NodePoolsDiffResult].
func NodePoolsDiff(old, new NodePoolsViewType) NodePoolsDiffResult {
	result := NodePoolsDiffResult{
		PartiallyDeleted: NodePoolsViewType{},
		Deleted:          NodePoolsViewType{},
		PartiallyAdded:   NodePoolsViewType{},
		Added:            NodePoolsViewType{},
	}

	// 1. track nodepools that are completely absent in the new version.
	for nodepool, nodes := range old {
		if _, ok := new[nodepool]; !ok {
			result.Deleted[nodepool] = nodes
		}
	}
	// delete the nodepools from the old version
	for nodepool := range result.Deleted {
		delete(old, nodepool)
	}

	// 2. track nodepools that are new in the new version.
	for nodepool, nodes := range new {
		if _, ok := old[nodepool]; !ok {
			result.Added[nodepool] = nodes
		}
	}
	// delete the nodepools from the new version.
	for nodepool := range result.Added {
		delete(new, nodepool)
	}

	// Now, In both old,new [NodePoolsViewType] we only have nodepools that are present in both.

	// 3. track partially deleted node from nodepools present in both versions.
	for nodepool, oldNodes := range old {
		newNodes := new[nodepool]

		for _, oldNode := range oldNodes {
			found := slices.Contains(newNodes, oldNode)
			if !found {
				result.PartiallyDeleted[nodepool] = append(result.PartiallyDeleted[nodepool], oldNode)
			}
		}
	}
	// delete the partially deleted nodes from the old version.
	for nodepool, nodes := range result.PartiallyDeleted {
		old[nodepool] = slices.DeleteFunc(old[nodepool], func(node string) bool { return slices.Contains(nodes, node) })
	}

	// 4. track partially added nodes from nodepools present in both versions.
	for nodepool, newNodes := range new {
		oldNodes := old[nodepool]

		for _, newNode := range newNodes {
			found := slices.Contains(oldNodes, newNode)
			if !found {
				result.PartiallyAdded[nodepool] = append(result.PartiallyAdded[nodepool], newNode)
			}
		}
	}
	// delete the partially added nodes from the old version.
	for nodepool, nodes := range result.PartiallyAdded {
		new[nodepool] = slices.DeleteFunc(new[nodepool], func(node string) bool { return slices.Contains(nodes, node) })
	}

	// Now in the new,old [NodePoolsViewType] only the nodes that are common to both of them remain,
	// but we do not work with those in here thus we leave them as is.
	return result
}

// NodePoolsView returns a view into the individual dynamic and static NodePools of the
// passed in [spec.K8Scluster] struct and returns them as an [NodePoolsViewType].
func NodePoolsView(cluster *spec.K8Scluster) (dynamic NodePoolsViewType, static NodePoolsViewType) {
	dynamic, static = make(NodePoolsViewType), make(NodePoolsViewType)

	for _, nodepool := range cluster.GetClusterInfo().GetNodePools() {
		switch np := nodepool.GetType().(type) {
		case *spec.NodePool_DynamicNodePool:
			// the nodepool can have 0 nodes, for example when
			// the autocaler scales from 0.
			dynamic[nodepool.Name] = []string{}

			for _, node := range nodepool.Nodes {
				dynamic[nodepool.Name] = append(dynamic[nodepool.Name], node.Name)
			}
		case *spec.NodePool_StaticNodePool:
			// The nodepool could have 0 nodes.
			static[nodepool.Name] = []string{}

			for _, node := range nodepool.Nodes {
				static[nodepool.Name] = append(static[nodepool.Name], node.Name)
			}
		default:
			// We panic here as this is an error from the programmer if any new
			// nodepool types should be added in the future.
			panic(fmt.Sprintf("Unsupported NodePool Type: %T", np))
		}
	}

	return
}
