package node_manager

import (
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/nodes"
	"github.com/berops/claudie/proto/pb/spec"

	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	defaultPodAmountsLimit = 110
)

type InstanceInfo struct {
	// Cpu cores
	cpu int64

	// Size in bytes
	memory int64

	// Size in bytes
	disk int64

	// Number of NVIDIA gpus
	nvidiaGpus int64
}

// NodeManager manages information about each node type
// such as CPU, Memory, GPUs etc.
type NodeManager struct {
	// VM type info caches
	hetznerVMs   map[string]*InstanceInfo
	gcpVMs       map[string]*InstanceInfo
	awsVMs       map[string]*InstanceInfo
	azureVMs     map[string]*InstanceInfo
	ociVMs       map[string]*InstanceInfo
	openstackVMs map[string]*InstanceInfo
	exoscaleVMs  map[string]*InstanceInfo

	// Provider-region-zone cache
	cacheProviderMap map[string]struct{}
	archResolver     *nodes.DynamicNodePoolResolver
}

// NewNodeManager returns a NodeManager pointer with initialised caches about nodes.
// Expects that the passed in nodepools are autoscaled nodepools.
func NewNodeManager() *NodeManager {
	return &NodeManager{
		cacheProviderMap: make(map[string]struct{}),
		archResolver:     nodes.NewDynamicNodePoolResolver(),
	}
}

// Refresh checks if the information about specified nodepools needs refreshing, and if so, refreshes it.
func (nm *NodeManager) Refresh(autoscaled []*spec.NodePool) error {
	for _, nodepool := range autoscaled {
		dyn := nodepool.GetDynamicNodePool()

		// Check if cache was already set.
		// Check together with region and zone as not all instances
		// are supported everywhere.
		providerId := fmt.Sprintf("%s-%s-%s", dyn.Provider.CloudProviderName, dyn.Region, dyn.Zone)

		if _, ok := nm.cacheProviderMap[providerId]; !ok {
			switch dyn.Provider.CloudProviderName {
			case "hetzner":
				if err := nm.cacheHetzner(dyn); err != nil {
					return err
				}
			case "aws":
				if err := nm.cacheAws(dyn); err != nil {
					return err
				}
			case "gcp":
				if err := nm.cacheGcp(dyn); err != nil {
					return err
				}
			case "oci":
				if err := nm.cacheOci(dyn); err != nil {
					return err
				}
			case "azure":
				if err := nm.cacheAzure(dyn); err != nil {
					return err
				}
			case "openstack":
				if err := nm.cacheOpenstack(dyn); err != nil {
					return err
				}
			case "exoscale":
				if err := nm.cacheExoscale(dyn); err != nil {
					return err
				}
			}
			// Save flag for this provider-region-zone combination.
			nm.cacheProviderMap[providerId] = struct{}{}
		}
	}
	return nil
}

// GetCapacity returns a theoretical capacity for a new node from specified nodepool.
func (nm *NodeManager) GetCapacity(np *spec.DynamicNodePool) k8sV1.ResourceList {
	instanceInfo := nm.InstanceInfo(np)
	if instanceInfo == nil {
		return nil
	}

	var disk int64
	// Check if disk is define for the instance.
	if instanceInfo.disk > 0 {
		disk = instanceInfo.disk
	} else {
		disk = int64(np.StorageDiskSize) * 1024 * 1024 * 1024 // Convert to bytes
	}

	rl := k8sV1.ResourceList{
		k8sV1.ResourcePods:    *resource.NewQuantity(defaultPodAmountsLimit, resource.DecimalSI),
		k8sV1.ResourceCPU:     *resource.NewQuantity(instanceInfo.cpu, resource.DecimalSI),
		k8sV1.ResourceMemory:  *resource.NewQuantity(instanceInfo.memory, resource.DecimalSI),
		k8sV1.ResourceStorage: *resource.NewQuantity(disk, resource.DecimalSI),
	}

	if instanceInfo.nvidiaGpus > 0 {
		rl["nvidia.com/gpu"] = *resource.NewQuantity(instanceInfo.nvidiaGpus, resource.DecimalSI)
	}

	// If the machine spec contains a valid number of NvidiaGPUs, prefer that value over the cached
	// one from [typeInfo].
	if np.MachineSpec != nil && np.MachineSpec.NvidiaGpuCount > 0 {
		rl["nvidia.com/gpu"] = *resource.NewQuantity(int64(np.MachineSpec.NvidiaGpuCount), resource.DecimalSI)
	}

	return rl
}

// Arch returns the architecture for the dynamic nodepool.
func (nm *NodeManager) Arch(np *spec.NodePool) (nodes.Arch, error) {
	return nm.archResolver.Arch(np)
}

// Returns cached InstanceInfo for the nodepool, nil if does not exist.
func (nm *NodeManager) InstanceInfo(np *spec.DynamicNodePool) *InstanceInfo {
	switch strings.ToLower(np.Provider.CloudProviderName) {
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
	case "openstack":
		if ti, ok := nm.openstackVMs[np.ServerType]; ok {
			return ti
		}
	case "exoscale":
		if ti, ok := nm.exoscaleVMs[np.ServerType]; ok {
			return ti
		}
	}
	return nil
}
