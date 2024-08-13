package utils

import (
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
)

// updateClusterInfo updates the desired state based on the current state
// namely:
// - Hash
// - AutoscalerConfig
// - existing nodes
// - nodepool
//   - metadata
//   - Public key
//   - Private key
func updateClusterInfo(desired, current *spec.ClusterInfo) error {
	desired.Hash = current.Hash
desired:
	for _, desiredNp := range desired.NodePools {
		for _, currentNp := range current.NodePools {
			if desiredNp.Name != currentNp.Name {
				continue
			}

			switch {
			case tryUpdateDynamicNodePool(desiredNp, currentNp):
			case tryUpdateStaticNodePool(desiredNp, currentNp):
			default:
				return fmt.Errorf("%q is neither dynamic nor static, unexpected value: %v", desiredNp.Name, desiredNp.GetNodePoolType())
			}

			continue desired
		}
	}

	return nil
}

func tryUpdateDynamicNodePool(desired, current *spec.NodePool) bool {
	dnp := desired.GetDynamicNodePool()
	cnp := current.GetDynamicNodePool()

	canUpdate := dnp != nil && cnp != nil
	if !canUpdate {
		return false
	}

	dnp.PublicKey = cnp.PublicKey
	dnp.PrivateKey = cnp.PrivateKey

	desired.Nodes = current.Nodes
	dnp.Cidr = cnp.Cidr

	// Update the count
	if cnp.AutoscalerConfig != nil && dnp.AutoscalerConfig != nil {
		// Both have Autoscaler conf defined, use same count as in current
		dnp.Count = cnp.Count
	} else if cnp.AutoscalerConfig == nil && dnp.AutoscalerConfig != nil {
		// Desired is autoscaled, but not current
		if dnp.AutoscalerConfig.Min > cnp.Count {
			// Cannot have fewer nodes than defined min
			dnp.Count = dnp.AutoscalerConfig.Min
		} else if dnp.AutoscalerConfig.Max < cnp.Count {
			// Cannot have more nodes than defined max
			dnp.Count = dnp.AutoscalerConfig.Max
		} else {
			// Use same count as in current for now, autoscaler might change it later
			dnp.Count = cnp.Count
		}
	}

	return true
}

func tryUpdateStaticNodePool(desired, current *spec.NodePool) bool {
	dnp := desired.GetStaticNodePool()
	cnp := current.GetStaticNodePool()

	canUpdate := dnp != nil && cnp != nil
	if !canUpdate {
		return false
	}

	for _, dn := range desired.Nodes {
		for _, cn := range current.Nodes {
			if dn.Public == cn.Public {
				dn.Name = cn.Name
				dn.Private = cn.Private
				dn.NodeType = cn.NodeType
			}
		}
	}

	return true
}
