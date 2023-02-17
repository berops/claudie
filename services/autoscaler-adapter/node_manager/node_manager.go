package node_manager

import (
	"context"
	"fmt"
	"strings"

	"github.com/berops/claudie/proto/pb"
	"github.com/hetznercloud/hcloud-go/hcloud"
	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	amd64                  = "amd64"
	arm64                  = "arm64"
	defaultPodAmountsLimit = 110
)

type NodeManager struct {
	hetznerVMs []*hcloud.ServerType
	// gcpVMs
	// awsVMS
	// azureVMs
	// ociVMs
}

type typeInfo struct {
	// cpu cores in milicores
	CPU int64
	// size in bytes
	Memory int64
	// size in bytes
	Disk int64
}

func NewNodeManager(nodepools []*pb.NodePool) *NodeManager {
	nm := &NodeManager{}
	cacheProviderMap := make(map[string]struct{})
	for _, np := range nodepools {
		if np.AutoscalerConfig != nil {
			// Check if cache was already set.
			if _, ok := cacheProviderMap[np.Provider.CloudProviderName]; !ok {
				switch np.Provider.CloudProviderName {
				case "hetzner":
					hc := hcloud.NewClient(hcloud.WithToken(np.Provider.Credentials))

					if servers, err := hc.ServerType.All(context.Background()); err != nil {
						panic(fmt.Sprintf("Hetzner client got error %v", err))
					} else {
						nm.hetznerVMs = servers
					}
				}
				cacheProviderMap[np.Provider.CloudProviderName] = struct{}{}
			}
		}
	}
	return nm
}

func (nm *NodeManager) GetOs(image string) string {
	// Only supported OS
	return "ubuntu"
}

func (nm *NodeManager) GetArch(np *pb.NodePool) string {
	switch np.Provider.CloudProviderName {
	case "hetzner":
		// Hetzner only provides amd64 VMs
		return amd64
	}
	return ""
}

func (nm *NodeManager) GetCapacity(np *pb.NodePool) k8sV1.ResourceList {
	switch np.Provider.CloudProviderName {
	case "hetzner":
		typeInfo := nm.getTypeInfo("hetzner", np)
		return k8sV1.ResourceList{
			k8sV1.ResourcePods:    *resource.NewQuantity(defaultPodAmountsLimit, resource.DecimalSI),
			k8sV1.ResourceCPU:     *resource.NewQuantity(typeInfo.CPU, resource.DecimalSI),
			k8sV1.ResourceMemory:  *resource.NewQuantity(typeInfo.Memory, resource.DecimalSI),
			k8sV1.ResourceStorage: *resource.NewQuantity(typeInfo.Disk, resource.DecimalSI),
		}
	}
	return nil
}

func (nm *NodeManager) GetLabels(np *pb.NodePool) map[string]string {
	m := make(map[string]string)
	// Claudie assigned labels.
	m["claudie.io/nodepool"] = np.Name
	m["claudie.io/provider"] = np.Provider.CloudProviderName
	m["claudie.io/provider-instance"] = np.Provider.SpecName
	m["claudie.io/worker-node"] = fmt.Sprintf("%v", !np.IsControl)
	m["topology.kubernetes.io/zone"] = np.Zone
	m["topology.kubernetes.io/region"] = np.Region
	// Other labels.
	m["kubernetes.io/os"] = "linux" // Only Linux is supported.
	m["v1.kubeone.io/operating-system"] = nm.GetOs(np.Image)
	m["kubernetes.io/arch"] = nm.GetArch(np)

	return m
}

func (nm *NodeManager) getTypeInfo(provider string, np *pb.NodePool) typeInfo {
	ti := typeInfo{}
	switch provider {
	case "hetzner":
		// cpx versions are called ccx in hcloud-go api.
		npType := strings.ReplaceAll(np.ServerType, "p", "c")
		for _, server := range nm.hetznerVMs {
			if server.Name == npType {
				ti.CPU = int64(server.Cores * 1000)
				ti.Memory = int64(server.Memory * 1024 * 1024 * 1024) // Convert to bytes
				ti.Disk = int64(server.Disk * 1024 * 1024 * 1024)     // Convert to bytes
			}
		}
	}
	return ti
}
