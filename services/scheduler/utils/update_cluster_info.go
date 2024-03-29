package utils

import "github.com/berops/claudie/proto/pb"

// updateClusterInfo updates the desired state based on the current state
// namely:
// - Hash
// - Public key
// - Private key
// - AutoscalerConfig
// - existing nodes
// - nodepool metadata
func updateClusterInfo(desired, current *pb.ClusterInfo) {
	desired.Hash = current.Hash
	desired.PublicKey = current.PublicKey
	desired.PrivateKey = current.PrivateKey
	// check for autoscaler configuration
desired:
	for _, desiredNp := range desired.NodePools {
		for _, currentNp := range current.NodePools {
			// Found nodepool in desired and in Current
			if desiredNp.Name == currentNp.Name {
				if dnp, cnp := getDynamicNodePools(desiredNp, currentNp); dnp != nil && cnp != nil {
					// Save current nodes and metadata
					desiredNp.Nodes = currentNp.Nodes
					dnp.Metadata = cnp.Metadata
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
					continue desired
				} else if dnp, cnp := getStaticNodePools(desiredNp, currentNp); dnp != nil && cnp != nil {
					// Found nodepool in desired and in Current
					for _, dn := range desiredNp.Nodes {
						for _, cn := range currentNp.Nodes {
							if dn.Public == cn.Public {
								dn.Name = cn.Name
								dn.Private = cn.Private
								dn.NodeType = cn.NodeType
							}
						}
					}
				}
			}
		}
	}
}

func getDynamicNodePools(np1, np2 *pb.NodePool) (*pb.DynamicNodePool, *pb.DynamicNodePool) {
	return np1.GetDynamicNodePool(), np2.GetDynamicNodePool()
}

func getStaticNodePools(np1, np2 *pb.NodePool) (*pb.StaticNodePool, *pb.StaticNodePool) {
	return np1.GetStaticNodePool(), np2.GetStaticNodePool()
}
