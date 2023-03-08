package node_manager

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/berops/claudie/proto/pb"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	defaultPodAmountsLimit = 110
)

type NodeManager struct {
	hetznerVMs map[string]*typeInfo
	gcpVMs     map[string]*typeInfo
	awsVMs     map[string]*typeInfo
	azureVMs   map[string]*typeInfo
	ociVMs     map[string]*typeInfo
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
func NewNodeManager(nodepools []*pb.NodePool) *NodeManager {
	nm := &NodeManager{}
	cacheProviderMap := make(map[string]struct{})
	for _, np := range nodepools {
		if np.AutoscalerConfig != nil {
			// Check if cache was already set. Check together with region and zone as not all instances can be supported everywhere.
			providerId := fmt.Sprintf("%s-%s-%s", np.Provider.CloudProviderName, np.Region, np.Zone)
			if _, ok := cacheProviderMap[providerId]; !ok {
				switch np.Provider.CloudProviderName {
				case "hetzner":
					// Create client and create cache.
					hc := hcloud.NewClient(hcloud.WithToken(np.Provider.Credentials), hcloud.WithHTTPClient(http.DefaultClient))
					if servers, err := hc.ServerType.All(context.Background()); err != nil {
						panic(fmt.Sprintf("Hetzner client got error %v", err))
					} else {
						nm.hetznerVMs = getTypeInfosHetzner(servers)
					}
				case "aws":
					// Define option function to set credentials
					credFunc := func(lo *config.LoadOptions) error {
						lo.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
							return aws.Credentials{AccessKeyID: np.Provider.AwsAccessKey, SecretAccessKey: np.Provider.Credentials}, nil
						})
						return nil
					}
					// Create client and create cache.
					httpClient := awshttp.NewBuildableClient().WithTransportOptions(func(tr *http.Transport) {
						if tr.TLSClientConfig == nil {
							tr.TLSClientConfig = &tls.Config{}
						}
						tr.TLSClientConfig.MinVersion = tls.VersionTLS12
					})
					cfg, err := config.LoadDefaultConfig(context.Background(), credFunc, config.WithHTTPClient(httpClient), config.WithRegion(np.Region))
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
							nm.awsVMs = mergeMaps(getTypeInfosAws(res.InstanceTypes), nm.awsVMs)
							// Check if there are any more results to query.
							token = res.NextToken
							if res.NextToken == nil {
								break
							}
						}
					}
				case "gcp":
					// Create client and create cache
					computeService, err := compute.NewMachineTypesRESTClient(context.Background(), option.WithCredentialsJSON([]byte(np.Provider.Credentials)))
					if err != nil {
						panic(fmt.Sprintf("GCP client got error : %v", err))
					}
					defer computeService.Close()
					// Define request and parameters
					maxResults := uint32(30)
					req := &computepb.ListMachineTypesRequest{
						Project:    np.Provider.GcpProject,
						MaxResults: &maxResults,
						Zone:       np.Zone,
					}
					// List services
					it := computeService.List(context.Background(), req)
					machineTypes := make([]*computepb.MachineType, 0)
					// Use while loop to support paging
					for {
						mt, err := it.Next()
						if err == iterator.Done {
							break
						}
						if err != nil {
							panic(fmt.Sprintf("GCP client got error while looping: %v", err))
						}
						machineTypes = append(machineTypes, mt)
					}
					nm.gcpVMs = mergeMaps(getTypeInfosGcp(machineTypes), nm.gcpVMs)

				case "oci":
					conf := common.NewRawConfigurationProvider(np.Provider.OciTenancyOcid, np.Provider.OciUserOcid, np.Region, np.Provider.OciFingerprint, np.Provider.Credentials, nil)
					client, err := core.NewComputeClientWithConfigurationProvider(conf)
					if err != nil {
						panic(fmt.Sprintf("OCI client got error : %v", err))
					}
					maxResults := 30
					req := core.ListShapesRequest{
						CompartmentId: &np.Provider.OciCompartmentOcid,
						Limit:         &maxResults,
					}
					for {
						r, err := client.ListShapes(context.Background(), req)
						if err != nil {
							panic(fmt.Sprintf("OCI client got error : %v", err))
						}
						if r.Items == nil || len(r.Items) == 0 {
							panic("OCI client got empty response")
						}
						nm.ociVMs = mergeMaps(getTypeInfosOci(r.Items), nm.ociVMs)
						if r.OpcNextPage != nil {
							req.Page = r.OpcNextPage
						} else {
							break
						}
					}
				case "azure":
					cred, err := azidentity.NewClientSecretCredential(np.Provider.AzureTenantId, np.Provider.AzureClientId, np.Provider.Credentials, nil)
					if err != nil {
						panic(fmt.Sprintf("Azure client got error : %v", err))
					}
					client, err := armcompute.NewVirtualMachineSizesClient(np.Provider.AzureSubscriptionId, cred, nil)
					if err != nil {
						panic(fmt.Sprintf("Azure client got error : %v", err))
					}
					location := strings.ToLower(strings.ReplaceAll(np.Region, " ", ""))
					pager := client.NewListPager(location, nil)
					for pager.More() {
						nextResult, err := pager.NextPage(context.Background())
						if err != nil {
							panic(fmt.Sprintf("Azure client got error : %v", err))
						}
						nm.azureVMs = mergeMaps(getTypeInfosAzure(nextResult.Value), nm.azureVMs)
					}
				}
				// Save flag for this provider-region-zone combination
				cacheProviderMap[providerId] = struct{}{}
			}
		}
	}
	return nm
}

func (nm *NodeManager) GetOs(image string) string {
	// Only supported OS
	return "ubuntu"
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
