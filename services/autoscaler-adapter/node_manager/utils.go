package node_manager

import (
	"strings"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// getTypeInfoHetzner converts []*hcloud.ServerType to typeInfo map of instances, where keys are instance types.
func getTypeInfoHetzner(rawInfo []*hcloud.ServerType) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, server := range rawInfo {
		// The cpx versions are called ccx in hcloud-go api.
		serverName := strings.ReplaceAll(server.Name, "ccx", "cpx")
		m[serverName] = &typeInfo{
			cpu:    int64(server.Cores),
			memory: int64(server.Memory * 1024 * 1024 * 1024), // Convert to bytes
			disk:   int64(server.Disk * 1024 * 1024 * 1024),   // Convert to bytes
		}
	}
	return m
}

// getTypeInfoAws converts []types.InstanceTypeInfo to typeInfo map of instances, where keys are instance types.
func getTypeInfoAws(rawInfo []types.InstanceTypeInfo) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, instance := range rawInfo {
		// Ignore disk as it is set on nodepool level
		serverName := string(instance.InstanceType)
		m[serverName] = &typeInfo{
			cpu:    int64(*instance.VCpuInfo.DefaultCores),
			memory: *instance.MemoryInfo.SizeInMiB * 1024 * 1024, // Convert to bytes
		}
	}
	return m
}

// getTypeInfoAws converts []*computepb.MachineTypeto typeInfo map of instances, where keys are instance types.
func getTypeInfoGcp(rawInfo []*computepb.MachineType) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, instance := range rawInfo {
		m[*instance.Name] = &typeInfo{
			cpu:    int64(*instance.GuestCpus),
			memory: int64(*instance.MemoryMb) * 1024 * 1024, // Convert to bytes
		}
	}
	return m
}

// getTypeInfoAws converts []core.Shape to typeInfo map of instances, where keys are instance types.
func getTypeInfoOci(rawInfo []core.Shape) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, shape := range rawInfo {
		m[*shape.Shape] = &typeInfo{
			cpu:    int64(*shape.Ocpus),
			memory: int64(*shape.MemoryInGBs) * 1024 * 1024 * 1024, // Convert to bytes
		}
	}
	return m
}

// getTypeInfoAws converts []*armcompute.VirtualMachineSize to typeInfo map of instances, where keys are instance types.
func getTypeInfoAzure(rawInfo []*armcompute.VirtualMachineSize) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, vm := range rawInfo {
		m[*vm.Name] = &typeInfo{
			cpu:    int64(*vm.NumberOfCores),
			memory: int64(*vm.MemoryInMB) * 1024 * 1024, // Convert to bytes
		}
	}
	return m
}

// mergeMaps merges two or more maps together, into single map.
func mergeMaps[M ~map[K]V, K comparable, V any](maps ...M) M {
	merged := make(M)
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}
