package service

import (
	"fmt"

	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"google.golang.org/protobuf/proto"
)

// Creates a new nodepool from the 'nodepool' reference passed in 'current'
// state [spec.Clusters]. The newly returned nodepool does not share any memory
// with any of the passed in parameters and can be use as standalone.
func CreateNodePoolForRollingUpdate(
	current *spec.Clusters,
	cid *LoadBalancerIdentifier, // optional, if set will look inside loadbalancer.
	nodepool string,
	newTemplates *spec.TemplateRepository,
) (*spec.NodePool, error) {
	clusterId := current.K8S.ClusterInfo.Id()
	search := current.K8S.ClusterInfo.NodePools
	if cid != nil {
		search = current.LoadBalancers.Clusters[cid.Index].ClusterInfo.NodePools
		clusterId = current.LoadBalancers.Clusters[cid.Index].ClusterInfo.Id()
	}

	reference := nodepools.FindByName(nodepool, search)
	newNodePool := proto.Clone(reference).(*spec.NodePool)

	// Collect all used nodepool names within the current state.
	// Names are unique within a cluster thus it is enough to
	// look for names in the 'search' nodepools, which will either
	// be loadbalancer cluster nodepools or kuberentes cluster nodepools.
	usedNames := make(map[string]struct{})

	for _, np := range search {
		usedNames[np.Name] = struct{}{}
	}

	// 1. Generate a New Name for the nodepool.
	n, _ := nodepools.MustExtractNameAndHash(newNodePool.Name)
	for {
		// In practice this should never infinite loop. the Hash
		// that is created is 7 chars long and has an alphabet size
		// of 36, 36^7 ~= 78,364,164,096 Which is unreasonable that
		// in practice that many nodepools would exist in a single cluster.
		name := fmt.Sprintf("%s-%s", n, hash.Create(hash.Length))
		if _, ok := usedNames[name]; !ok {
			usedNames[name] = struct{}{}
			newNodePool.Name = name
			break
		}
	}

	newNodePool.Nodes = nil

	inner := newNodePool.GetDynamicNodePool()
	inner.PublicKey = ""
	inner.PrivateKey = ""
	inner.Cidr = ""

	// 2. Generate new keys.
	if err := generateSSHKeys(newNodePool); err != nil {
		return nil, err
	}

	// 3. Generate new CIDRs
	{
		wantedDesiredState := proto.Clone(current).(*spec.Clusters)

		if cid == nil {
			wantedDesiredState.K8S.ClusterInfo.NodePools = append(wantedDesiredState.K8S.ClusterInfo.NodePools, newNodePool)
		} else {
			lb := wantedDesiredState.LoadBalancers.Clusters[cid.Index]
			lb.ClusterInfo.NodePools = append(lb.ClusterInfo.NodePools, newNodePool)
		}

		if err := generateMissingCIDR(current, wantedDesiredState); err != nil {
			return nil, err
		}
	}

	// 4. Replace Templates
	inner.Provider.Templates = proto.Clone(newTemplates).(*spec.TemplateRepository)

	// 5. Generate Nodes.
	PopulateDynamicNodes(clusterId, newNodePool)

	// NOTE: any additional steps will need to be mirrored here if any are added in the [createDesiredState] function.

	return newNodePool, nil
}
