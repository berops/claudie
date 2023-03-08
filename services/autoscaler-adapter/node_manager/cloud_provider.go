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
	"github.com/rs/zerolog/log"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	defaultMaxResults = 30
)

// cacheHetzner function uses hcloud-go module to query supported servers and their info. If the query is successful, the server info is saved in cache.
func (nm *NodeManager) cacheHetzner(np *pb.NodePool) {
	// Create client and create cache.
	hc := hcloud.NewClient(hcloud.WithToken(np.Provider.Credentials), hcloud.WithHTTPClient(http.DefaultClient))
	if servers, err := hc.ServerType.All(context.Background()); err != nil {
		panic(fmt.Sprintf("Hetzner client got error %v", err))
	} else {
		nm.hetznerVMs = getTypeInfosHetzner(servers)
	}
}

// cacheAws function uses aws-sdk-go-v2 module to query supported VMs and their info. If the query is successful, the VM info is saved in cache.
func (nm *NodeManager) cacheAws(np *pb.NodePool) {
	// Define option function to set static credentials
	credFunc := func(lo *config.LoadOptions) error {
		lo.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: np.Provider.AwsAccessKey, SecretAccessKey: np.Provider.Credentials}, nil
		})
		return nil
	}

	// Create http client.
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
	maxResults := int32(defaultMaxResults)
	var token *string
	// Use while loop to support paging.
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
}

// cacheGcp function uses google go module to query supported VMs and their info. If the query is successful, the VM info is saved in cache.
func (nm *NodeManager) cacheGcp(np *pb.NodePool) {
	// Create client and create cache
	computeService, err := compute.NewMachineTypesRESTClient(context.Background(), option.WithCredentialsJSON([]byte(np.Provider.Credentials)))
	if err != nil {
		panic(fmt.Sprintf("GCP client got error : %v", err))
	}
	defer func() {
		if err := computeService.Close(); err != nil {
			log.Error().Msgf("Failed to close GCP client")
		}
	}()
	// Define request and parameters
	maxResults := uint32(defaultMaxResults)
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
}

// cacheOci function uses oci-go-sdk module to query supported shapes and their info. If the query is successful, the shape info is saved in cache.
func (nm *NodeManager) cacheOci(np *pb.NodePool) {
	conf := common.NewRawConfigurationProvider(np.Provider.OciTenancyOcid, np.Provider.OciUserOcid, np.Region, np.Provider.OciFingerprint, np.Provider.Credentials, nil)
	client, err := core.NewComputeClientWithConfigurationProvider(conf)
	if err != nil {
		panic(fmt.Sprintf("OCI client got error : %v", err))
	}
	maxResults := defaultMaxResults
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
}

// cacheOci function uses oci-go-sdk module to query supported VMs and their info. If the query is successful, the VM info is saved in cache.
func (nm *NodeManager) cacheAzure(np *pb.NodePool) {
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
