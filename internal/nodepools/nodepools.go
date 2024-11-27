package nodepools

import (
	"maps"
	"slices"

	"github.com/berops/claudie/proto/pb/spec"
)

// FindByName returns the first Nodepool that will have same name as specified in parameters, nil otherwise.
func FindByName(nodePoolName string, nodePools []*spec.NodePool) *spec.NodePool {
	for _, np := range nodePools {
		if np.Name == nodePoolName {
			return np
		}
	}
	return nil
}

// ExtractRegions will return a list of all regions used in list of nodepools
func ExtractRegions(nodepools []*spec.DynamicNodePool) []string {
	// create a set of region
	set := make(map[string]struct{})
	for _, nodepool := range nodepools {
		set[nodepool.Region] = struct{}{}
	}
	return slices.Collect(maps.Keys(set))
}

// ExtractDynamic returns slice of dynamic node pools.
func ExtractDynamic(nodepools []*spec.NodePool) []*spec.DynamicNodePool {
	dnps := make([]*spec.DynamicNodePool, 0, len(nodepools))
	for _, np := range nodepools {
		if n := np.GetDynamicNodePool(); n != nil {
			dnps = append(dnps, n)
		}
	}
	return dnps
}

// Dynamic returns every dynamic nodepool.
func Dynamic(nodepools []*spec.NodePool) []*spec.NodePool {
	dynamic := make([]*spec.NodePool, 0, len(nodepools))
	for _, n := range nodepools {
		if n.GetDynamicNodePool() != nil {
			dynamic = append(dynamic, n)
		}
	}
	return dynamic
}

// Static returns every static nodepool.
func Static(nodepools []*spec.NodePool) []*spec.NodePool {
	static := make([]*spec.NodePool, 0, len(nodepools))
	for _, n := range nodepools {
		if n.GetStaticNodePool() != nil {
			static = append(static, n)
		}
	}
	return static
}

func CommonDynamicNodes(currentNp, desiredNp []*spec.NodePool) []*spec.NodePool {
	dynamic := make(map[string]*spec.NodePool)
	for _, np := range currentNp {
		if np.GetDynamicNodePool() != nil {
			dynamic[np.Name] = np
		}
	}

	return commonNodes(dynamic, desiredNp)
}

func CommonStaticNodes(currentNp, desiredNp []*spec.NodePool) []*spec.NodePool {
	static := make(map[string]*spec.NodePool)
	for _, np := range currentNp {
		if np.GetStaticNodePool() != nil {
			static[np.Name] = np
		}
	}

	return commonNodes(static, desiredNp)
}

func commonNodes(currControlNps map[string]*spec.NodePool, desiredNp []*spec.NodePool) []*spec.NodePool {
	var commonNps []*spec.NodePool

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
				commonNps = append(commonNps, commonNodePool)
			}
		}
	}

	return commonNps
}
