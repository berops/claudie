package nodepools

import (
	"fmt"
	"iter"

	"github.com/berops/claudie/proto/pb/spec"
)

// ByProviderSpecName returns an iterator that groups nodepools by provider SpecName.
func ByProviderSpecName(nodepools []*spec.NodePool) iter.Seq2[string, []*spec.NodePool] {
	sortedNodePools := map[string][]*spec.NodePool{}

	for _, nodepool := range nodepools {
		if np := nodepool.GetDynamicNodePool(); np != nil {
			sortedNodePools[np.Provider.SpecName] = append(sortedNodePools[np.Provider.SpecName], nodepool)
		} else if np := nodepool.GetStaticNodePool(); np != nil {
			sortedNodePools[spec.StaticNodepoolInfo_STATIC_PROVIDER.String()] = append(sortedNodePools[spec.StaticNodepoolInfo_STATIC_PROVIDER.String()], nodepool)
		}
	}

	return func(yield func(string, []*spec.NodePool) bool) {
		for k, v := range sortedNodePools {
			if !yield(k, v) {
				return
			}
		}
	}
}

// ByProviderRegion returns an iterator that groups nodepools by provider region.
func ByProviderRegion(nodepools []*spec.NodePool) iter.Seq2[string, []*spec.NodePool] {
	sortedNodePools := map[string][]*spec.NodePool{}
	for _, nodepool := range nodepools {
		if np := nodepool.GetDynamicNodePool(); np != nil {
			key := fmt.Sprintf("%s-%s", np.Provider.SpecName, np.Region)
			sortedNodePools[key] = append(sortedNodePools[key], nodepool)
		} else if np := nodepool.GetStaticNodePool(); np != nil {
			key := fmt.Sprintf("%s-%s", spec.StaticNodepoolInfo_STATIC_PROVIDER.String(), spec.StaticNodepoolInfo_STATIC_REGION.String())
			sortedNodePools[key] = append(sortedNodePools[key], nodepool)
		}
	}

	return func(yield func(string, []*spec.NodePool) bool) {
		for k, v := range sortedNodePools {
			if !yield(k, v) {
				return
			}
		}
	}
}

func Control(nodepools []*spec.NodePool) iter.Seq[*spec.NodePool] {
	return func(yield func(*spec.NodePool) bool) {
		for _, np := range nodepools {
			if np.IsControl {
				if !yield(np) {
					return
				}
			}
		}
	}
}
