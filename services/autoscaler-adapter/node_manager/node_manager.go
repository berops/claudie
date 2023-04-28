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

// GetOs returns operating system name as a string.
func (nm *NodeManager) GetOs(image string) string {
	// Only supported OS
	return "ubuntu"
}

// GetCapacity returns a theoretical capacity for a new node from specified nodepool.
func (nm *NodeManager) GetCapacity(np *pb.NodePool) k8sV1.ResourceList {
	typeInfo := nm.getTypeInfo(np.Provider.CloudProviderName, np)
	if typeInfo != nil {
		var disk int64
		// Check if disk is define for the instance.
		if typeInfo.disk > 0 {
			disk = typeInfo.disk
		} else {
			disk = int64(np.StorageDiskSize) * 1024 * 1024 * 1024 // Convert to bytes
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

// GetLabels returns default labels with their theoretical values for the specified nodepool.
func (nm *NodeManager) GetLabels(np *pb.NodePool) map[string]string {
	m := make(map[string]string)
	// Claudie assigned labels.
	m["claudie.io/nodepool"] = np.Name
	m["claudie.io/provider"] = np.Provider.CloudProviderName
	m["claudie.io/provider-instance"] = np.Provider.SpecName
	m["claudie.io/node-type"] = getNodeType(np)
	m["topology.kubernetes.io/zone"] = np.Zone
	m["topology.kubernetes.io/region"] = np.Region
	// Other labels.
	m["kubernetes.io/os"] = "linux" // Only Linux is supported.
	//m["kubernetes.io/arch"] = "" // TODO add arch
	m["v1.kubeone.io/operating-system"] = nm.GetOs(np.Image)

	return m
}

// getTypeInfo returns a typeInfo for this nodepool
func (nm *NodeManager) getTypeInfo(provider string, np *pb.NodePool) *typeInfo {
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
	for _, np := range nps {
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
