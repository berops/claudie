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
				// Save current nodes and metadata
				desiredNp.Nodes = currentNp.Nodes
				desiredNp.Metadata = currentNp.Metadata
				// Update the count
				if currentNp.AutoscalerConfig != nil && desiredNp.AutoscalerConfig != nil {
					// Both have Autoscaler conf defined, use same count as in current
					desiredNp.Count = currentNp.Count
				} else if currentNp.AutoscalerConfig == nil && desiredNp.AutoscalerConfig != nil {
					// Desired is autoscaled, but not current
					if desiredNp.AutoscalerConfig.Min > currentNp.Count {
						// Cannot have fewer nodes than defined min
						desiredNp.Count = desiredNp.AutoscalerConfig.Min
					} else if desiredNp.AutoscalerConfig.Max < currentNp.Count {
						// Cannot have more nodes than defined max
						desiredNp.Count = desiredNp.AutoscalerConfig.Max
					} else {
						// Use same count as in current for now, autoscaler might change it later
						desiredNp.Count = currentNp.Count
					}
				}
				continue desired
			}
		}
	}
}
