package node_manager

import (
	"regexp"
	"strconv"
	"strings"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/oracle/oci-go-sdk/v65/core"
)

var (
	genesisTypeInfoRe = regexp.MustCompile(`vcpu-(\d+)_memory-(\d+)g`)
)

func getTypeInfoGenesisCloud() (map[string]*typeInfo, error) {
	// TODO: once there is an api exposed via the genesiscloud-go client
	// (https://github.com/genesiscloud/genesiscloud-go) rewrite this.
	// Types fetched from: https://developers.genesiscloud.com/instance-types/
	instanceTypes := []string{
		// CPU Instances AMD EPYC™
		"vcpu-2_memory-4g",
		"vcpu-4_memory-8g",
		"vcpu-8_memory-16g",
		"vcpu-12_memory-24g",
		"vcpu-16_memory-32g",
		"vcpu-20_memory-40g",
		"vcpu-24_memory-48g",

		// GPU Instances NVIDIA® GeForce™ RTX 3090
		"vcpu-4_memory-24g_nvidia-rtx-3090-1",
		"vcpu-8_memory-48g_nvidia-rtx-3090-2",
		"vcpu-12_memory-72g_nvidia-rtx-3090-3",
		"vcpu-16_memory-96g_nvidia-rtx-3090-4",
		"vcpu-20_memory-120g_nvidia-rtx-3090-5",
		"vcpu-24_memory-144g_nvidia-rtx-3090-6",
		"vcpu-28_memory-168g_nvidia-rtx-3090-7",
		"vcpu-32_memory-192g_nvidia-rtx-3090-8",

		// GPU Instances NVIDIA® GeForce™ RTX 3090 - CPU & Memory Optimized
		"vcpu-8_memory-48g_nvidia-rtx-3090-1",
		"vcpu-16_memory-96g_nvidia-rtx-3090-2",
		"vcpu-24_memory-144g_nvidia-rtx-3090-3",
		"vcpu-32_memory-192g_nvidia-rtx-3090-4",

		// GPU Instances NVIDIA® GeForce™ RTX 3080
		"vcpu-4_memory-16g_nvidia-rtx-3080-1",
		"vcpu-8_memory-32g_nvidia-rtx-3080-2",
		"vcpu-12_memory-48g_nvidia-rtx-3080-3",
		"vcpu-16_memory-64g_nvidia-rtx-3080-4",
		"vcpu-20_memory-80g_nvidia-rtx-3080-5",
		"vcpu-24_memory-96g_nvidia-rtx-3080-6",
		"vcpu-28_memory-112g_nvidia-rtx-3080-7",
		"vcpu-32_memory-128g_nvidia-rtx-3080-8",

		// GPU Instances NVIDIA® GeForce™ RTX 3080 - CPU & Memory Optimized
		"vcpu-8_memory-32g_nvidia-rtx-3080-1",
		"vcpu-16_memory-64g_nvidia-rtx-3080-2",
		"vcpu-24_memory-96g_nvidia-rtx-3080-3",
		"vcpu-32_memory-128g_nvidia-rtx-3080-4",

		// Accelerator Instances NVIDIA® H100 SXM5 80 GB
		"vcpu-192_memory-1920g_nvidia-h100-sxm5-8",

		// Accelerator Instances Intel® Gaudi®2
		"vcpu-16_memory-120g_intel-gaudi2-1",
		"vcpu-32_memory-240g_intel-gaudi2-2",
		"vcpu-48_memory-360g_intel-gaudi2-3",
		"vcpu-64_memory-480g_intel-gaudi2-4",
		"vcpu-80_memory-600g_intel-gaudi2-5",
		"vcpu-96_memory-720g_intel-gaudi2-6",
		"vcpu-112_memory-840g_intel-gaudi2-7",
		"vcpu-128_memory-960g_intel-gaudi2-8",

		// GPU Instances NVIDIA® GeForce™ GTX 1080 Ti
		"vcpu-4_memory-12g_nvidia-gtx-1080ti-1",
		"vcpu-8_memory-24g_nvidia-gtx-1080ti-2",
		"vcpu-12_memory-36g_nvidia-gtx-1080ti-3",
		"vcpu-16_memory-48g_nvidia-gtx-1080ti-4",
		"vcpu-20_memory-60g_nvidia-gtx-1080ti-5",
		"vcpu-24_memory-72g_nvidia-gtx-1080ti-6",
		"vcpu-28_memory-84g_nvidia-gtx-1080ti-7",
		"vcpu-32_memory-96g_nvidia-gtx-1080ti-8",

		// Legacy CPU Instances AMD EPYC™
		"vcpu-2_memory-4g_disk-80g",
		"vcpu-4_memory-8g_disk-80g",
		"vcpu-8_memory-16g_disk-80g",
		"vcpu-12_memory-24g_disk-80g",
		"vcpu-16_memory-32g_disk-80g",
		"vcpu-20_memory-40g_disk-80g",
		"vcpu-24_memory-48g_disk-80g",

		// Legacy GPU Instances NVIDIA® GeForce™ RTX 3090
		"vcpu-4_memory-18g_disk-80g_nvidia3090-1",
		"vcpu-8_memory-36g_disk-80g_nvidia3090-2",
		"vcpu-12_memory-54g_disk-80g_nvidia3090-3",
		"vcpu-16_memory-72g_disk-80g_nvidia3090-4",
		"vcpu-20_memory-90g_disk-80g_nvidia3090-5",
		"vcpu-24_memory-108g_disk-80g_nvidia3090-6",
		"vcpu-28_memory-126g_disk-80g_nvidia3090-7",
		"vcpu-32_memory-144g_disk-80g_nvidia3090-8",

		// Legacy GPU Instances NVIDIA® GeForce™ RTX 3090
		"vcpu-4_memory-24g_disk-80g_nvidia3090-1",
		"vcpu-8_memory-48g_disk-80g_nvidia3090-2",
		"vcpu-12_memory-72g_disk-80g_nvidia3090-3",
		"vcpu-16_memory-96g_disk-80g_nvidia3090-4",
		"vcpu-20_memory-120g_disk-80g_nvidia3090-5",
		"vcpu-24_memory-144g_disk-80g_nvidia3090-6",
		"vcpu-28_memory-168g_disk-80g_nvidia3090-7",
		"vcpu-32_memory-192g_disk-80g_nvidia3090-8",

		// Legacy GPU Instances NVIDIA® GeForce™ RTX 3090 - CPU & Memory Optimized
		"vcpu-8_memory-48g_disk-80g_nvidia3090-1",
		"vcpu-16_memory-96g_disk-80g_nvidia3090-2",
		"vcpu-24_memory-144g_disk-80g_nvidia3090-3",
		"vcpu-32_memory-192g_disk-80g_nvidia3090-4",

		// Legacy GPU Instances NVIDIA® GeForce™ RTX 3080
		"vcpu-4_memory-12g_disk-80g_nvidia3080-1",
		"vcpu-8_memory-24g_disk-80g_nvidia3080-2",
		"vcpu-12_memory-36g_disk-80g_nvidia3080-3",
		"vcpu-16_memory-48g_disk-80g_nvidia3080-4",
		"vcpu-20_memory-60g_disk-80g_nvidia3080-5",
		"vcpu-24_memory-72g_disk-80g_nvidia3080-6",
		"vcpu-28_memory-84g_disk-80g_nvidia3080-7",
		"vcpu-32_memory-96g_disk-80g_nvidia3080-8",

		// Legacy GPU Instances NVIDIA® GeForce™ RTX 3080 - CPU & Memory Optimized
		"vcpu-8_memory-32g_disk-80g_nvidia3080-1",
		"vcpu-16_memory-64g_disk-80g_nvidia3080-2",
		"vcpu-24_memory-96g_disk-80g_nvidia3080-3",
		"vcpu-32_memory-128g_disk-80g_nvidia3080-4",

		// Legacy GPU Instances NVIDIA® GeForce™ GTX 1080 Ti
		"vcpu-4_memory-12g_disk-80g_nvidia1080ti-1",
		"vcpu-8_memory-24g_disk-80g_nvidia1080ti-2",
		"vcpu-12_memory-36g_disk-80g_nvidia1080ti-3",
		"vcpu-16_memory-48g_disk-80g_nvidia1080ti-4",
		"vcpu-20_memory-60g_disk-80g_nvidia1080ti-5",
		"vcpu-24_memory-72g_disk-80g_nvidia1080ti-6",
		"vcpu-28_memory-84g_disk-80g_nvidia1080ti-7",
		"vcpu-32_memory-96g_disk-80g_nvidia1080ti-8",
	}

	m := make(map[string]*typeInfo, len(instanceTypes))
	for _, typ := range instanceTypes {
		serverName := typ
		matched := genesisTypeInfoRe.FindStringSubmatch(serverName)
		vcpus, err := strconv.ParseInt(matched[1], 10, 64)
		if err != nil {
			return nil, err
		}
		memoryGB, err := strconv.ParseInt(matched[2], 10, 64)
		if err != nil {
			return nil, err
		}

		m[serverName] = &typeInfo{
			cpu:    vcpus,
			memory: memoryGB * (1024 * 1024 * 1024), // convert to bytes,
		}
	}

	return m, nil
}

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
