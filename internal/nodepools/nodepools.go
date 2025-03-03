package nodepools

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"
)

func DeleteByName(nodepools []*spec.NodePool, name string) []*spec.NodePool {
	for i, np := range nodepools {
		if np.Name == name {
			return slices.Delete(nodepools, i, i+1)
		}
	}
	return nodepools
}

// DeleteNodeByName goes through each nodepool until if find the node with the specified name. If the nodepool
// reaches 0 nodes the keepNodePools map is checked whether the nodepool should be removed or not.
func DeleteNodeByName(nodepools []*spec.NodePool, nodeName string, keepNodePools map[string]struct{}) []*spec.NodePool {
	for n, np := range nodepools {
		j := slices.IndexFunc(np.Nodes, func(n *spec.Node) bool { return n.Name == nodeName })
		if j < 0 {
			continue
		}
		if s := np.GetStaticNodePool(); s != nil {
			delete(s.NodeKeys, np.Nodes[j].Public)
		}
		if d := np.GetDynamicNodePool(); d != nil {
			d.Count -= 1
		}
		np.Nodes = slices.Delete(np.Nodes, j, j+1)

		if len(np.Nodes) == 0 {
			if _, ok := keepNodePools[np.Name]; !ok {
				return slices.Delete(nodepools, n, n+1)
			}
		}
		break
	}

	return nodepools
}

func FindNode(nodepools []*spec.NodePool, nodeName string) (static bool, node *spec.Node) {
	for _, np := range nodepools {
		i := slices.IndexFunc(np.Nodes, func(n *spec.Node) bool { return n.Name == nodeName })
		if i < 0 {
			continue
		}
		static = np.GetStaticNodePool() != nil
		node = np.Nodes[i]
		return
	}
	return
}

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

func MatchNameAndHashWithTemplate(template, nodepoolName string) (n, h string) {
	if len(nodepoolName) != len(template)+hash.Length+1 {
		return
	}

	idx := strings.LastIndex(nodepoolName, "-")
	if idx < 0 {
		return "", ""
	}

	if nodepoolName[:idx] != template {
		return
	}

	n = nodepoolName[:idx]
	h = nodepoolName[idx+1:]

	return
}

func MustExtractNameAndHash(pool string) (name, hash string) {
	idx := strings.LastIndex(pool, "-")
	if idx < 0 {
		panic("this function expect that the nodepool name contains a appended hash delimited by '-'")
	}

	name = pool[:idx]
	hash = pool[idx+1:]

	return name, hash
}

// FindApiEndpoint searches for a nodepool that has the control node representing the Api endpoint of the cluster.
func FindApiEndpoint(nodepools []*spec.NodePool) (*spec.NodePool, *spec.Node) {
	for _, np := range nodepools {
		if np.IsControl {
			if node := np.EndpointNode(); node != nil {
				return np, node
			}
		}
	}
	return nil, nil
}

// FirstControlNode returns the first control node encountered.
func FirstControlNode(nodepools []*spec.NodePool) *spec.Node {
	for _, np := range nodepools {
		for _, node := range np.Nodes {
			if node.NodeType == spec.NodeType_master {
				return node
			}
		}
	}
	return nil
}

// DynamicGenerateKeys creates private keys files for all nodes in the provided dynamic node pools in form
// of <node name>.pem.
func DynamicGenerateKeys(nodepools []*spec.NodePool, outputDir string) error {
	errs := make([]error, 0, len(nodepools))
	for _, dnp := range nodepools {
		pk := dnp.GetDynamicNodePool().PrivateKey
		if err := fileutils.CreateKey(pk, outputDir, fmt.Sprintf("%s.pem", dnp.Name)); err != nil {
			errs = append(errs, fmt.Errorf("%q failed to create key file: %w", dnp.Name, err))
		}
	}
	return errors.Join(errs...)
}

// StaticGenerateKeys creates private keys files for all nodes in the provided static node pools in form
// of <node name>.pem.
func StaticGenerateKeys(nodepools []*spec.NodePool, outputDir string) error {
	errs := make([]error, 0, len(nodepools))
	for _, staticNp := range nodepools {
		sp := staticNp.GetStaticNodePool()
		for _, node := range staticNp.Nodes {
			if key, ok := sp.NodeKeys[node.Public]; ok {
				if err := fileutils.CreateKey(key, outputDir, fmt.Sprintf("%s.pem", node.Name)); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}
	// If empty, returns nil
	return errors.Join(errs...)
}
