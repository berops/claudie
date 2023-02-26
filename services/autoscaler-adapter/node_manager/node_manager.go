package node_manager

import (
	"context"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
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
	// TODO merge them to single cache map
	hetznerVMs map[string]*typeInfo
	gcpVMs     map[string]*typeInfo
	awsVMs     map[string]*typeInfo
	// azureVMs
	// ociVMs
}

type typeInfo struct {
	// Cpu cores in milicores
	cpu int64
	// Size in bytes
	memory int64
	// Size in bytes
	disk int64
	// Architecture
	arch string
}

// NewNodeManager returns a NodeManager pointer with initialised caches about nodes.
func NewNodeManager(nodepools []*pb.NodePool) *NodeManager {
	nm := &NodeManager{}
	cacheProviderMap := make(map[string]struct{})
	for _, np := range nodepools {
		if np.AutoscalerConfig != nil {
			// Check if cache was already set.
			// TODO check regions as well
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
					// Use while loop to support paging
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
					// Create client and create cache
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
					// Use while loop to support paging
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
					// TODO
				case "azure":
					// TODO
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
			return t.arch
		}
	case "gcp":
		//TODO
		return ""
	}
	return ""
}

// GetCapacity returns a theoretical capacity for a new node from specified nodepool.
func (nm *NodeManager) GetCapacity(np *pb.NodePool) k8sV1.ResourceList {
	typeInfo := nm.getTypeInfo(np.Provider.CloudProviderName, np)
	if typeInfo != nil {
		var disk int64
		if typeInfo.disk > 0 {
			disk = typeInfo.disk
		} else {
			disk = int64(np.DiskSize) * 1024 * 1024 * 1024 // Convert to bytes
		}
		return k8sV1.ResourceList{
			k8sV1.ResourcePods:    *resource.NewQuantity(defaultPodAmountsLimit, resource.DecimalSI),
			k8sV1.ResourceCPU:     *resource.NewQuantity(typeInfo.cpu, resource.DecimalSI),
			k8sV1.ResourceMemory:  *resource.NewQuantity(typeInfo.memory, resource.DecimalSI),
			k8sV1.ResourceStorage: *resource.NewQuantity(disk, resource.DecimalSI),
		}
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
	m["v1.kubeone.io/operating-system"] = nm.GetOs(np.Image)
	m["kubernetes.io/arch"] = nm.GetArch(np)

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
	}
	return nil
}
