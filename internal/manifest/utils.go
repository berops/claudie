package manifest

import (
	"fmt"

	"github.com/Berops/claudie/proto/pb"
)

// GetProvider will search for a Provider config by matching name from providerSpec
// returns *pb.Provider,nil if matching Provider config found otherwise returns nil,error
func (ds *Manifest) GetProvider(providerSpecName string) (*pb.Provider, error) {
	for _, gcpConf := range ds.Providers.GCP {
		if gcpConf.Name == providerSpecName {
			return &pb.Provider{
				SpecName:          gcpConf.Name,
				Credentials:       gcpConf.Credentials,
				GcpProject:        gcpConf.GCPProject,
				CloudProviderName: "gcp",
			}, nil
		}
	}

	for _, hetznerConf := range ds.Providers.Hetzner {
		if hetznerConf.Name == providerSpecName {
			return &pb.Provider{
				SpecName:          hetznerConf.Name,
				Credentials:       hetznerConf.Credentials,
				CloudProviderName: "hetzner",
			}, nil
		}
	}

	return nil, fmt.Errorf("failed to find provider with name: %s", providerSpecName)
}

// IsKubernetesClusterPresent checks in the manifests if a cluster
// was defined with the specified name.
func (m *Manifest) IsKubernetesClusterPresent(name string) bool {
	for _, c := range m.Kubernetes.Clusters {
		if c.Name == name {
			return true
		}
	}

	return false
}

// FindNodePool will search for the nodepool in manifest.DynamicNodePool based on the nodepool name
// returns *manifest.DynamicNodePool if found, nil otherwise
func (ds *Manifest) FindNodePool(nodePoolName string) *DynamicNodePool {
	for _, nodePool := range ds.NodePools.Dynamic {
		if nodePool.Name == nodePoolName {
			return &nodePool
		}
	}
	return nil
}

// CreateNodepools will create a pb.Nodepool structs based on the manifest specification
// returns error if nodepool/provider not defined, nil otherwise
func (ds *Manifest) CreateNodepools(pools []string, isControl bool) ([]*pb.NodePool, error) {
	var nodePools []*pb.NodePool
	for _, nodePoolName := range pools {
		// Check if the nodepool is part of the cluster
		var nodePool *DynamicNodePool = ds.FindNodePool(nodePoolName)
		if nodePool != nil {
			provider, err := ds.GetProvider(nodePool.ProviderSpec.Name)
			if err != nil {
				return nil, err
			}

			nodePools = append(nodePools, &pb.NodePool{
				Name:       nodePool.Name,
				Region:     nodePool.ProviderSpec.Region,
				Zone:       nodePool.ProviderSpec.Zone,
				ServerType: nodePool.ServerType,
				Image:      nodePool.Image,
				DiskSize:   uint32(nodePool.DiskSize),
				Count:      uint32(nodePool.Count),
				Provider:   provider,
				IsControl:  isControl,
			})
		} else {
			return nil, fmt.Errorf("nodepool %s not defined", nodePoolName)
		}
	}
	return nodePools, nil
}
