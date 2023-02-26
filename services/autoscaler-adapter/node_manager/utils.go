package node_manager

import (
	"strings"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/berops/claudie/proto/pb"
	"github.com/hetznercloud/hcloud-go/hcloud"
)

func getTypeInfosHetzner(rawInfo []*hcloud.ServerType) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, server := range rawInfo {
		ti := &typeInfo{}
		ti.cpu = int64(server.Cores)                          // Convert to milicores
		ti.memory = int64(server.Memory * 1024 * 1024 * 1024) // Convert to bytes
		ti.disk = int64(server.Disk * 1024 * 1024 * 1024)     // Convert to bytes
		// The cpx versions are called ccx in hcloud-go api.
		serverName := strings.ReplaceAll(server.Name, "ccx", "cpx")
		m[serverName] = ti
	}
	return m
}

func getTypeInfosAws(rawInfo []types.InstanceTypeInfo) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, instance := range rawInfo {
		ti := &typeInfo{}
		ti.cpu = int64(*instance.VCpuInfo.DefaultCores)
		ti.memory = *instance.MemoryInfo.SizeInMiB * 1024 * 1024
		ti.arch = getAwsNodeArch(instance.ProcessorInfo.SupportedArchitectures)
		// Ignore disk as it is set on nodepool level
		serverName := string(instance.InstanceType)
		m[serverName] = ti
	}
	return m
}

func getTypeInfosGcp(rawInfo []*computepb.MachineType) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, instance := range rawInfo {
		ti := &typeInfo{}
		ti.cpu = int64(*instance.GuestCpus)
		ti.memory = int64(*instance.MemoryMb * 1000 * 1000)
		ti.arch = "" //TODO
		m[*instance.Name] = ti
	}
	return m
}

func mergeMaps[M ~map[K]V, K comparable, V any](maps ...M) M {
	merged := make(M)
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

func getAwsNodeArch(archs []types.ArchitectureType) string {
	if strings.Contains(string(archs[0]), "x86") {
		return amd64
	} else if strings.Contains(string(archs[0]), "i386") {
		return "i386"
	}
	return arm64
}

func getNodeType(np *pb.NodePool) string {
	if np.IsControl {
		return "control"
	}
	return "compute"
}
