package node_manager

import (
	"context"
	"fmt"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/berops/claudie/proto/pb"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	amd64                  = "amd64"
	arm64                  = "arm64"
	defaultPodAmountsLimit = 110
)

type NodeManager struct {
	hetznerVMs map[string]*typeInfo
	gcpVMs     map[string]*typeInfo
	awsVMs     map[string]*typeInfo
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
	// architecture
	Arch string
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
					// Create client and create cache.
					hc := hcloud.NewClient(hcloud.WithToken(np.Provider.Credentials))
					if servers, err := hc.ServerType.All(context.Background()); err != nil {
						panic(fmt.Sprintf("Hetzner client got error %v", err))
					} else {
						nm.hetznerVMs = getTypeInfosHetzner(servers)
					}
				case "aws":
					// Define option function to set credentials
					optFunc := func(lo *config.LoadOptions) error {
						lo.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
							return aws.Credentials{AccessKeyID: np.Provider.AwsAccessKey, SecretAccessKey: np.Provider.Credentials}, nil
						})
						lo.Region = np.Region
						return nil
					}
					// Create client and create cache.
					cfg, err := config.LoadDefaultConfig(context.Background(), optFunc)
					if err != nil {
						panic(fmt.Sprintf("AWS config got error : %v", err))
					}
					client := ec2.NewFromConfig(cfg)
					maxResults := int32(30)
					var token *string
					for {
						if res, err := client.DescribeInstanceTypes(context.Background(), &ec2.DescribeInstanceTypesInput{MaxResults: &maxResults, NextToken: token}); err != nil {
							panic(fmt.Sprintf("AWS client got error : %v", err))
						} else {
							nm.awsVMs = mergeMaps(nm.awsVMs, getTypeInfosAws(res.InstanceTypes))
							// Check if there are any more results to query.
							token = res.NextToken
							if res.NextToken == nil {
								break
							}
						}
					}
				case "gcp":
					computeService, err := compute.NewMachineTypesRESTClient(context.Background(), option.WithAPIKey(np.Provider.Credentials))
					if err != nil {
						panic(fmt.Sprintf("GCP client got error : %v", err))
					}
					defer computeService.Close()
					maxResults := uint32(30)

					req := &computepb.ListMachineTypesRequest{
						Project:    np.Provider.GcpProject,
						MaxResults: &maxResults,
					}
					it := computeService.List(context.Background(), req)
					machineTypes := make([]*computepb.MachineType, 0)
					for {
						mt, err := it.Next()
						if err == iterator.Done {
							break
						}
						if err != nil {
							panic(fmt.Sprintf("GCP client got error : %v", err))
						}
						machineTypes = append(machineTypes, mt)
					}
					nm.gcpVMs = mergeMaps(nm.gcpVMs, getTypeInfosGcp(machineTypes))

				case "oci":

				case "azure":
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
	case "aws":
		if t, ok := nm.awsVMs[np.ServerType]; ok {
			return t.Arch
		}
	}
	return ""
}

func (nm *NodeManager) GetCapacity(np *pb.NodePool) k8sV1.ResourceList {
	typeInfo := nm.getTypeInfo(np.Provider.CloudProviderName, np)
	if typeInfo != nil {
		var disk int64
		if typeInfo.Disk > 0 {
			disk = typeInfo.Disk
		} else {
			disk = int64(np.DiskSize) * 1024 * 1024 * 1024 // Convert to bytes
		}
		return k8sV1.ResourceList{
			k8sV1.ResourcePods:    *resource.NewQuantity(defaultPodAmountsLimit, resource.DecimalSI),
			k8sV1.ResourceCPU:     *resource.NewQuantity(typeInfo.CPU, resource.DecimalSI),
			k8sV1.ResourceMemory:  *resource.NewQuantity(typeInfo.Memory, resource.DecimalSI),
			k8sV1.ResourceStorage: *resource.NewQuantity(disk, resource.DecimalSI),
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

func (nm *NodeManager) getTypeInfo(provider string, np *pb.NodePool) *typeInfo {
	ti := typeInfo{}
	switch provider {
	case "hetzner":
		if server, ok := nm.hetznerVMs[np.ServerType]; ok {
			return server
		}

	case "aws":
		if instance, ok := nm.awsVMs[np.ServerType]; ok {
			return instance
		}
	}
	return &ti
}

func getTypeInfosHetzner(rawInfo []*hcloud.ServerType) map[string]*typeInfo {
	m := make(map[string]*typeInfo, len(rawInfo))
	for _, server := range rawInfo {
		ti := &typeInfo{}
		ti.CPU = int64(server.Cores)                          // Convert to milicores
		ti.Memory = int64(server.Memory * 1024 * 1024 * 1024) // Convert to bytes
		ti.Disk = int64(server.Disk * 1024 * 1024 * 1024)     // Convert to bytes
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
		ti.CPU = int64(*instance.VCpuInfo.DefaultCores)
		ti.Memory = *instance.MemoryInfo.SizeInMiB * 1024 * 1024
		ti.Arch = getNodeArch(instance.ProcessorInfo.SupportedArchitectures)
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
		ti.CPU = int64(*instance.GuestCpus)
		ti.Memory = int64(*instance.MemoryMb * 1000 * 1000)
		ti.Arch = "" //TODO
		m[*instance.Name] = ti
	}
	return m
}

func mergeMaps[M ~map[K]V, K comparable, V any](src ...M) M {
	merged := make(M)
	for _, m := range src {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

func getNodeArch(archs []types.ArchitectureType) string {
	if strings.Contains(string(archs[0]), "x86") {
		return amd64
	} else if strings.Contains(string(archs[0]), "i386") {
		return "i386"
	}
	return arm64
}
