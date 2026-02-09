package node_manager

import (
	"strings"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
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
			cpu:        int64(server.Cores),
			memory:     int64(server.Memory * 1024 * 1024 * 1024), // Convert to bytes
			disk:       int64(server.Disk * 1024 * 1024 * 1024),   // Convert to bytes
			nvidiaGpus: 0,                                         // hetzner doesn't have any GPU instances at the time of implementing this.
		}
	}
	return m
}

// getTypeInfoAws converts []types.InstanceTypeInfo to typeInfo map of instances, where keys are instance types.
func getTypeInfoAws(rawInfo []types.InstanceTypeInfo) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, instance := range rawInfo {
		info := &typeInfo{
			cpu:        int64(*instance.VCpuInfo.DefaultCores),
			memory:     *instance.MemoryInfo.SizeInMiB * 1024 * 1024, // Convert to bytes
			nvidiaGpus: 0,
		}
		if gpus := instance.GpuInfo; gpus != nil {
			for _, gpu := range gpus.Gpus {
				manufacturer, count := "", int64(0)

				if gpu.Manufacturer != nil {
					manufacturer = strings.ToLower(*gpu.Manufacturer)
				}
				if gpu.Count != nil {
					count = int64(*gpu.Count)
				}
				if !strings.Contains(manufacturer, "nvidia") || count <= 0 {
					continue
				}

				info.nvidiaGpus += count
			}
		}
		// Ignore disk as it is set on nodepool level
		serverName := string(instance.InstanceType)
		m[serverName] = info
	}
	return m
}

// getTypeInfoGcp converts []*computepb.MachineTypeto typeInfo map of instances, where keys are instance types.
func getTypeInfoGcp(rawInfo []*computepb.MachineType) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, instance := range rawInfo {
		info := &typeInfo{
			cpu:        int64(*instance.GuestCpus),
			memory:     int64(*instance.MemoryMb) * 1024 * 1024, // Convert to bytes
			nvidiaGpus: 0,
		}
		// Seems like there are accelerators assigned to the given machine type, but the
		// requests return always empty slices. In GCP there are only a few instance types
		// that support GPUs, but one can select a combination of 1,2,4,8 GPUs which there
		// seems to not be and API for query for.
		// https://stackoverflow.com/questions/76782409/gcp-rest-call-to-get-machine-types-supported-for-different-gpu-types
		//
		// If the API returns a slice of GPUs, use it, otherwise this will be a noop and in
		// that case the the user would need to manually give us this information in the
		// inputmanifest via the [manifest.NodePool.MachineSpec.NvidiaGpuCount].
		for _, acc := range instance.Accelerators {
			manufacturer, count := "", int64(0)

			if acc.GuestAcceleratorType != nil {
				manufacturer = strings.ToLower(*acc.GuestAcceleratorType)
			}
			if acc.GuestAcceleratorCount != nil {
				count = int64(*acc.GuestAcceleratorCount)
			}
			if !strings.Contains(manufacturer, "nvidia") || count <= 0 {
				continue
			}

			info.nvidiaGpus += count
		}
		m[*instance.Name] = info
	}
	return m
}

// getTypeInfoAws converts []core.Shape to typeInfo map of instances, where keys are instance types.
func getTypeInfoOci(rawInfo []core.Shape) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, shape := range rawInfo {
		info := &typeInfo{
			cpu:    int64(*shape.Ocpus),
			memory: int64(*shape.MemoryInGBs) * 1024 * 1024 * 1024, // Convert to bytes
			// while the SKD does provide the number of GPUs attached to the instance via
			// [core.Shape.GpuCount], there seems to be no API to provide explicit vendor metadata,
			//  for example to say if the GPU is NVIDIA.
			nvidiaGpus: 0,
		}
		m[*shape.Shape] = info
	}
	return m
}

// getTypeInfoAws converts []*armcompute.VirtualMachineSize to typeInfo map of instances, where keys are instance types.
func getTypeInfoAzure(rawInfo []*armcompute.VirtualMachineSize) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, vm := range rawInfo {
		info := &typeInfo{
			cpu:    int64(*vm.NumberOfCores),
			memory: int64(*vm.MemoryInMB) * 1024 * 1024, // Convert to bytes
			// while the SDK does provider an API for retrieving the number of GPUs for a type
			// there seems to be no API to provide explicit vendor metadata, for example if the GPU
			// is NVIDIA.
			nvidiaGpus: 0,
		}
		m[*vm.Name] = info
	}
	return m
}

func getTypeInfoOpenstack(rawInfo []flavors.Flavor) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, flavor := range rawInfo {
		m[flavor.Name] = &typeInfo{
			cpu:    int64(flavor.VCPUs),
			memory: int64(flavor.RAM) * 1024 * 1024, // Convert to bytes
		}
	}
	return m
}
