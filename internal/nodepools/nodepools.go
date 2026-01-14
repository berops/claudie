package nodepools

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"

	"google.golang.org/protobuf/proto"
)

type RegionNetwork struct {
	Region          string
	ExternalNetwork string
}

func DeleteByName(nodepools []*spec.NodePool, name string) []*spec.NodePool {
	for i, np := range nodepools {
		if np.Name == name {
			return slices.Delete(nodepools, i, i+1)
		}
	}
	return nodepools
}

// DeleteNodeByName goes through each nodepool until it finds the node with the specified name.
// If the nodepool reaches 0 nodes the keepNodePools map is checked whether the nodepool should
// be removed or not.
func DeleteNodeByName(
	nodepools []*spec.NodePool,
	nodeName string,
	keepNodePools map[string]struct{},
) []*spec.NodePool {
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

func NodeCount(nodepools []*spec.NodePool) int {
	var out int

	for _, np := range nodepools {
		switch i := np.Type.(type) {
		case *spec.NodePool_DynamicNodePool:
			out += int(i.DynamicNodePool.Count)
		case *spec.NodePool_StaticNodePool:
			out += len(i.StaticNodePool.NodeKeys)
		}
	}

	return out
}

// Adds the passed in `nodes` into `dst` by also correctly increasing
// the count of the nodes within the nodepool. The nodes are not cloned
// but simply shallow copied into the nodepool. If the nodepool is not
// dynamic this is a noop function.
func DynamicAddNodes(dst *spec.NodePool, nodes []*spec.Node) {
	dyn := dst.GetDynamicNodePool()
	if dyn == nil {
		return
	}

	dst.Nodes = append(dst.Nodes, nodes...)
	dyn.Count += int32(len(nodes))
}

// Behaves exactly the same as [PartialCopyWithNodeFilter] just that the
// passed in nodes are excluded from ones in the nodepool if present.
func PartialCopyWithNodeExclusion(np *spec.NodePool, nodes []string) *spec.NodePool {
	cp := &spec.NodePool{
		Type:        nil,
		Name:        np.Name,
		Nodes:       []*spec.Node{},
		IsControl:   np.IsControl,
		Labels:      np.Labels,
		Taints:      np.Taints,
		Annotations: np.Annotations,
	}

	for _, n := range np.Nodes {
		if !slices.Contains(nodes, n.Name) {
			cp.Nodes = append(cp.Nodes, n)
		}
	}

	// To avoid issues with possible node counts, deep clone the node type itself.
	switch typ := np.Type.(type) {
	case *spec.NodePool_DynamicNodePool:
		d := proto.Clone(typ.DynamicNodePool).(*spec.DynamicNodePool)
		d.Count = int32(len(cp.Nodes))
		cp.Type = &spec.NodePool_DynamicNodePool{
			DynamicNodePool: d,
		}
	case *spec.NodePool_StaticNodePool:
		s := proto.Clone(typ.StaticNodePool).(*spec.StaticNodePool)
		clear(s.NodeKeys)
		for _, n := range cp.Nodes {
			s.NodeKeys[n.Public] = np.GetStaticNodePool().NodeKeys[n.Public]
		}
		cp.Type = &spec.NodePool_StaticNodePool{
			StaticNodePool: s,
		}
	}

	return cp
}

// For each node in the nodepool that is found inside the passed in
// nodes slice, returns a shallow copy of the nodepool, meaning that
// all of the memory is still shared among the original and returned
// nodepool, but will only have the filtered nodes.
//
// **Caution** the Node Type itself is deep cloned as the node counts
// need to change to reflect the filered nodes.
func PartialCopyWithNodeFilter(np *spec.NodePool, nodes []string) *spec.NodePool {
	cp := &spec.NodePool{
		Type:        nil,
		Name:        np.Name,
		Nodes:       []*spec.Node{},
		IsControl:   np.IsControl,
		Labels:      np.Labels,
		Taints:      np.Taints,
		Annotations: np.Annotations,
	}

	for _, n := range np.Nodes {
		if slices.Contains(nodes, n.Name) {
			cp.Nodes = append(cp.Nodes, n)
		}
	}

	// To avoid issues with possible node counts, deep clone the node type itself.
	switch typ := np.Type.(type) {
	case *spec.NodePool_DynamicNodePool:
		d := proto.Clone(typ.DynamicNodePool).(*spec.DynamicNodePool)
		d.Count = int32(len(cp.Nodes))
		cp.Type = &spec.NodePool_DynamicNodePool{
			DynamicNodePool: d,
		}
	case *spec.NodePool_StaticNodePool:
		s := proto.Clone(typ.StaticNodePool).(*spec.StaticNodePool)
		clear(s.NodeKeys)
		for _, n := range cp.Nodes {
			s.NodeKeys[n.Public] = np.GetStaticNodePool().NodeKeys[n.Public]
		}
		cp.Type = &spec.NodePool_StaticNodePool{
			StaticNodePool: s,
		}
	}

	return cp
}

// All of the nodes in the nodepool are replaced with the passed in nodes slice.
// The function will create a shallow copy of the nodepool, meaning that all of
// the memory is still shared among the original and returned nodepool, but will
// only have the replaced nodes. If the nodes are static nodes it is expected that
// the passed in nodeKeys map will be filled with the required data.
//
// **Caution** the node Type itself is deep cloned as the node counts need
// to change to reflect the filtered nodes.
func PartialCopyWithReplacedNodes(np *spec.NodePool, nodes []*spec.Node, nodeKeys map[string]string) *spec.NodePool {
	cp := &spec.NodePool{
		Type:        nil,
		Name:        np.Name,
		Nodes:       nodes,
		IsControl:   np.IsControl,
		Labels:      np.Labels,
		Taints:      np.Taints,
		Annotations: np.Annotations,
	}

	// To avoid issues with possible node counts, deep clone the node type itself.
	switch typ := np.Type.(type) {
	case *spec.NodePool_DynamicNodePool:
		d := proto.Clone(typ.DynamicNodePool).(*spec.DynamicNodePool)
		d.Count = int32(len(cp.Nodes))
		cp.Type = &spec.NodePool_DynamicNodePool{
			DynamicNodePool: d,
		}
	case *spec.NodePool_StaticNodePool:
		s := proto.Clone(typ.StaticNodePool).(*spec.StaticNodePool)
		clear(s.NodeKeys)
		maps.Copy(s.NodeKeys, nodeKeys)
	}

	return cp
}

// Copies the nodes from `src` into `dst` cloning the invidivual
// nodes, such that they do not keep any pointers or shared
// memory with the original. The type of the nodepool of the
// `src` and `dst` must be the same, otherwise no copying is done.
//
// The affected src [NodePool] is modified with adjusted counts from
// the newly copied over nodes.
func CopyNodes(dst, src *spec.NodePool, nodes []string) {
	noop := src.GetDynamicNodePool() != nil && dst.GetDynamicNodePool() == nil
	noop = noop || (src.GetStaticNodePool() != nil && dst.GetStaticNodePool() == nil)
	if noop {
		return
	}

	for _, n := range src.Nodes {
		if !slices.Contains(nodes, n.Name) {
			continue
		}

		n := proto.Clone(n).(*spec.Node)
		dst.Nodes = append(dst.Nodes, n)

		switch src := src.GetType().(type) {
		case *spec.NodePool_DynamicNodePool:
			dst.GetDynamicNodePool().Count += 1
		case *spec.NodePool_StaticNodePool:
			dst.GetStaticNodePool().NodeKeys[n.Public] = src.StaticNodePool.NodeKeys[n.Public]
		}
	}
}

// Clones all of the nodes from [spec.NodePool] which have a
// matching name in the passed in nodes slice.
func CloneTargetNodes(n *spec.NodePool, nodes []string) []*spec.Node {
	var out []*spec.Node

	for _, n := range n.Nodes {
		if !slices.Contains(nodes, n.Name) {
			continue
		}

		n := proto.Clone(n).(*spec.Node)
		out = append(out, n)
	}

	return out
}

// Deletes any matching `nodes` in the passed in `nodepool`
// If the passed in `nodes` contain all of the nodes of the
// `nodepool` then the nodepool will be modified to contain
// no nodes at all.
func DeleteNodes(nodepool *spec.NodePool, nodes []string) {
	var deleted []*spec.Node
	nodepool.Nodes = slices.DeleteFunc(nodepool.Nodes, func(n *spec.Node) bool {
		if slices.Contains(nodes, n.Name) {
			deleted = append(deleted, n)
			return true
		}
		return false
	})

	switch np := nodepool.GetType().(type) {
	case *spec.NodePool_DynamicNodePool:
		np.DynamicNodePool.Count -= int32(len(deleted))
	case *spec.NodePool_StaticNodePool:
		for _, deleted := range deleted {
			delete(np.StaticNodePool.NodeKeys, deleted.Public)
		}
	}

	clear(deleted)
}

// Returns true if the node is within one of the provided nodepools.
func ContainsNode(nodepools []*spec.NodePool, nodeName string) bool {
	for _, np := range nodepools {
		for _, node := range np.Nodes {
			if node.Name == nodeName {
				return true
			}
		}
	}

	return false
}

func FindNode(nodepools []*spec.NodePool, nodeName string) (nodepool *spec.NodePool, node *spec.Node) {
	for _, np := range nodepools {
		i := slices.IndexFunc(np.Nodes, func(n *spec.Node) bool { return n.Name == nodeName })
		if i < 0 {
			continue
		}
		nodepool = np
		node = np.Nodes[i]
		return
	}
	return
}

// IndexByName returns the position of the nodepool within the slice, if not found -1 is returned.
func IndexByName(nodePoolName string, nodepools []*spec.NodePool) int {
	for i, np := range nodepools {
		if np.Name == nodePoolName {
			return i
		}
	}

	return -1
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

// ExtractRegionNetwork will return a unique list of all regions with networks used in list of nodepools
func ExtractRegionNetwork(nodepools []*spec.DynamicNodePool) []RegionNetwork {
	set := make(map[RegionNetwork]struct{})
	for _, nodepool := range nodepools {
		key := RegionNetwork{
			Region:          nodepool.Region,
			ExternalNetwork: nodepool.ExternalNetworkName,
		}
		set[key] = struct{}{}
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

// Autoscaled returns all autoscaled nodepools.
func Autoscaled(nodepools []*spec.NodePool) []*spec.NodePool {
	var autoscaled []*spec.NodePool

	for _, np := range nodepools {
		if n := np.GetDynamicNodePool(); n != nil {
			if n.AutoscalerConfig != nil {
				autoscaled = append(autoscaled, np)
			}
		}
	}

	return autoscaled
}

// Returns true if the nodepool is autoscaled.
func IsAutoscaled(np *spec.NodePool) bool {
	if n := np.GetDynamicNodePool(); n != nil {
		if n.AutoscalerConfig != nil {
			return true
		}
	}
	return false
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
		if np.IsControl && len(np.Nodes) > 0 {
			return np.Nodes[0]
		}
	}
	return nil
}

// Returns a random node. Nil if there is none.
func RandomNode(nodepools iter.Seq[*spec.NodePool]) *spec.Node {
	var nodes []*spec.Node

	for np := range nodepools {
		nodes = append(nodes, np.Nodes...)
	}

	if len(nodes) == 0 {
		return nil
	}

	idx := rand.IntN(len(nodes))
	return nodes[idx]
}

// Returns a random node public Endpoint and a SSH key to connect to it. Nil if there is none.
func RandomNodePublicEndpoint(nps []*spec.NodePool) (string, string, string) {
	if len(nps) == 0 {
		return "", "", ""
	}

	idx := rand.IntN(len(nps))
	np := nps[idx]

	if len(np.Nodes) == 0 {
		return "", "", ""
	}

	idx = rand.IntN(len(np.Nodes))
	node := np.Nodes[idx]

	endpoint := node.Public
	username := "root"
	if node.Username != "" && node.Username != username {
		username = node.Username
	}

	switch np := np.Type.(type) {
	case *spec.NodePool_DynamicNodePool:
		return username, endpoint, np.DynamicNodePool.PrivateKey
	case *spec.NodePool_StaticNodePool:
		return username, endpoint, np.StaticNodePool.NodeKeys[node.Public]
	default:
		return "", "", ""
	}
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

// TODO: remove.
type LabelsTaintsAnnotationsData struct {
	LabelKeys      map[string][]string
	AnnotationKeys map[string][]string
	TaintKeys      map[string][]*spec.Taint
}

func LabelsTaintsAnnotationsDiff(current, desired []*spec.NodePool) LabelsTaintsAnnotationsData {
	out := LabelsTaintsAnnotationsData{
		LabelKeys:      map[string][]string{},
		AnnotationKeys: map[string][]string{},
		TaintKeys:      map[string][]*spec.Taint{},
	}

	// No modifications are done just a comparison of missing annotations/labels/taints.
	cnp := make(map[string]*spec.NodePool)
	for _, np := range current {
		cnp[np.Name] = np
	}

	for _, desired := range desired {
		current, ok := cnp[desired.Name]
		if !ok {
			continue
		}
		// No need to check if the nodepool in current is missing from desired, because if thats
		// the case then we don't need to remove the labels/annotations/taints as all of the nodes
		// are to be removed anyways. We only look for keys that are missing in desired, new or
		// existing one will be created/updated.

		for k := range current.Labels {
			if _, ok := desired.Labels[k]; !ok {
				out.LabelKeys[desired.Name] = append(out.LabelKeys[desired.Name], k)
			}
		}

		for k := range current.Annotations {
			if _, ok := desired.Annotations[k]; !ok {
				out.AnnotationKeys[desired.Name] = append(out.AnnotationKeys[desired.Name], k)
			}
		}

		for _, t := range current.Taints {
			matchTaint := func(other *spec.Taint) bool {
				return other.Key == t.Key && other.Value == t.Value && other.Effect == t.Effect
			}
			if ok := slices.ContainsFunc(desired.Taints, matchTaint); !ok {
				out.TaintKeys[desired.Name] = append(out.TaintKeys[desired.Name], &spec.Taint{
					Key:    t.Key,
					Value:  t.Value,
					Effect: t.Effect,
				})
			}
		}
	}

	return out
}

// FindReferences return all nodepools that share the given name.
func FindReferences(name string, nodePools []*spec.NodePool) []*spec.NodePool {
	var references []*spec.NodePool
	for _, np := range nodePools {
		if np.Name == name {
			references = append(references, np)
		}
	}
	return references
}

// Returns all Public Endpoints, for all of the nodes for the passed in nodepools.
func PublicEndpoints(nodepools []*spec.NodePool) []string {
	var ips []string

	for _, nodepool := range nodepools {
		for _, node := range nodepool.Nodes {
			ips = append(ips, node.Public)
		}
	}

	return ips
}
