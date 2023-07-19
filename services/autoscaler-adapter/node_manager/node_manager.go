package node_manager

import (
	"fmt"

	"github.com/berops/claudie/proto/pb"
	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	defaultPodAmountsLimit = 110
)

type Arch string

// CPU architectures 64bit.
const (
	x86 Arch = "amd64"
	Arm Arch = "arm64"
)

type NodeManager struct {
	// VM type info caches
	hetznerVMs map[string]*typeInfo
	gcpVMs     map[string]*typeInfo
	awsVMs     map[string]*typeInfo
	azureVMs   map[string]*typeInfo
	ociVMs     map[string]*typeInfo
	// Provider-region-zone cache
	cacheProviderMap map[string]struct{}
}

type typeInfo struct {
	// Cpu cores
	cpu int64
	// Size in bytes
	memory int64
	// Size in bytes
	disk int64
	// arch of VM
	arch Arch
}

// NewNodeManager returns a NodeManager pointer with initialised caches about nodes.
func NewNodeManager(nodepools []*pb.NodePool) (*NodeManager, error) {
	nm := &NodeManager{}
	nm.cacheProviderMap = make(map[string]struct{})
	if err := nm.refreshCache(nodepools); err != nil {
		return nil, err
	}
	return nm, nil
}

// Refresh checks if the information about specified nodepools needs refreshing, and if so, refreshes it.
func (nm *NodeManager) Refresh(nodepools []*pb.NodePool) error {
	return nm.refreshCache(nodepools)
}

// GetCapacity returns a theoretical capacity for a new node from specified nodepool.
func (nm *NodeManager) GetCapacity(np *pb.NodePool) k8sV1.ResourceList {
	typeInfo := nm.getTypeInfo(np.GetDynamicNodePool().Provider.CloudProviderName, np.GetDynamicNodePool())
	if typeInfo != nil {
		var disk int64
		// Check if disk is define for the instance.
		if typeInfo.disk > 0 {
			disk = typeInfo.disk
		} else {
			disk = int64(np.GetDynamicNodePool().StorageDiskSize) * 1024 * 1024 * 1024 // Convert to bytes
		}
		rl := k8sV1.ResourceList{}
		rl[k8sV1.ResourcePods] = *resource.NewQuantity(defaultPodAmountsLimit, resource.DecimalSI)
		rl[k8sV1.ResourceCPU] = *resource.NewQuantity(typeInfo.cpu, resource.DecimalSI)
		rl[k8sV1.ResourceMemory] = *resource.NewQuantity(typeInfo.memory, resource.DecimalSI)
		rl[k8sV1.ResourceStorage] = *resource.NewQuantity(disk, resource.DecimalSI)
		return rl
	}
	return nil
}

// Return the Arch for the dynamic nodepool.
func (nm *NodeManager) QueryArch(np *pb.DynamicNodePool) Arch {
	return nm.getTypeInfo(np.GetProvider().CloudProviderName, np).arch
}

// getTypeInfo returns a typeInfo for this nodepool
func (nm *NodeManager) getTypeInfo(provider string, np *pb.DynamicNodePool) *typeInfo {
	switch provider {
	case "hetzner":
		if ti, ok := nm.hetznerVMs[np.ServerType]; ok {
			return ti
		}
	case "aws":
		if ti, ok := nm.awsVMs[np.ServerType]; ok {
			return ti
		}
	case "gcp":
		if ti, ok := nm.gcpVMs[np.ServerType]; ok {
			return ti
		}
	case "oci":
		if ti, ok := nm.ociVMs[np.ServerType]; ok {
			return ti
		}
	case "azure":
		if ti, ok := nm.azureVMs[np.ServerType]; ok {
			return ti
		}
	}
	return nil
}

// refreshCache refreshes node info cache if needed.
func (nm *NodeManager) refreshCache(nps []*pb.NodePool) error {
	for _, nodepool := range nps {
		np := nodepool.GetDynamicNodePool()
		if np == nil {
			continue
		}
		// Cache only for nodepools, which are autoscaled.
		if np.AutoscalerConfig != nil {
			// Check if cache was already set.
			// Check together with region and zone as not all instances
			// are be supported everywhere.
			providerId := fmt.Sprintf("%s-%s-%s", np.Provider.CloudProviderName, np.Region, np.Zone)
			if _, ok := nm.cacheProviderMap[providerId]; !ok {
				switch np.Provider.CloudProviderName {
				case "hetzner":
					if err := nm.cacheHetzner(np); err != nil {
						return err
					}
				case "aws":
					if err := nm.cacheAws(np); err != nil {
						return err
					}
				case "gcp":
					if err := nm.cacheGcp(np); err != nil {
						return err
					}
				case "oci":
					if err := nm.cacheOci(np); err != nil {
						return err
					}
				case "azure":
					if err := nm.cacheAzure(np); err != nil {
						return err
					}
				}
				// Save flag for this provider-region-zone combination.
				nm.cacheProviderMap[providerId] = struct{}{}
			}
		}
	}
	return nil
}
