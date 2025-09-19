package node_manager

import (
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/nodes"
	"github.com/berops/claudie/proto/pb/spec"

	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	defaultPodAmountsLimit = 110
)

type NodeManager struct {
	// VM type info caches
	genesisCloudVMs map[string]*typeInfo
	hetznerVMs      map[string]*typeInfo
	gcpVMs          map[string]*typeInfo
	awsVMs          map[string]*typeInfo
	azureVMs        map[string]*typeInfo
	ociVMs          map[string]*typeInfo
	// Provider-region-zone cache
	cacheProviderMap map[string]struct{}
	resolver         *nodes.DynamicNodePoolResolver
}

type typeInfo struct {
	// Cpu cores
	cpu int64
	// Size in bytes
	memory int64
	// Size in bytes
	disk int64
}

// NewNodeManager returns a NodeManager pointer with initialised caches about nodes.
func NewNodeManager(nps []*spec.NodePool) (*NodeManager, error) {
	nm := &NodeManager{}
	nm.cacheProviderMap = make(map[string]struct{})

	var err error

	nm.resolver, err = nodes.NewDynamicNodePoolResolver(nodepools.ExtractDynamic(nps))
	if err != nil {
		return nil, err
	}

	if err = nm.refreshCache(nps); err != nil {
		return nil, err
	}
	return nm, nil
}

// Refresh checks if the information about specified nodepools needs refreshing, and if so, refreshes it.
func (nm *NodeManager) Refresh(nodepools []*spec.NodePool) error {
	return nm.refreshCache(nodepools)
}

// GetCapacity returns a theoretical capacity for a new node from specified nodepool.
func (nm *NodeManager) GetCapacity(np *spec.NodePool) k8sV1.ResourceList {
	dnp := np.GetDynamicNodePool()
	if dnp == nil {
		return nil
	}

	typeInfo := nm.getTypeInfo(dnp.Provider.CloudProviderName, dnp)
	if typeInfo == nil {
		return nil
	}

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

	// If the machine spec contains a valid number of NvidiaGPUs, prefer that value over the cached
	// one from [typeInfo].
	if dnp.MachineSpec != nil && dnp.MachineSpec.NvidiaGpu > 0 {
		rl["nvidia.com/gpu"] = *resource.NewQuantity(int64(dnp.MachineSpec.NvidiaGpu), resource.DecimalSI)
	}
	return rl
}

// Arch returns the architecture for the dynamic nodepool.
func (nm *NodeManager) Arch(np *spec.NodePool) (nodes.Arch, error) {
	return nm.resolver.Arch(np)
}

// getTypeInfo returns a typeInfo for this nodepool
func (nm *NodeManager) getTypeInfo(provider string, np *spec.DynamicNodePool) *typeInfo {
	switch strings.ToLower(provider) {
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
	case "genesiscloud":
		if ti, ok := nm.genesisCloudVMs[np.ServerType]; ok {
			return ti
		}
	}
	return nil
}

// refreshCache refreshes node info cache if needed.
func (nm *NodeManager) refreshCache(nps []*spec.NodePool) error {
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
				case "genesiscloud":
					if err := nm.cacheGenesisCloud(np); err != nil {
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
