package node_manager

import (
	"context"
	"crypto/tls"
	"errors"
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
func (nm *NodeManager) cacheHetzner(np *pb.NodePool) error {
	// Create client and create cache.
	hc := hcloud.NewClient(hcloud.WithToken(np.Provider.Credentials), hcloud.WithHTTPClient(http.DefaultClient))
	if servers, err := hc.ServerType.All(context.Background()); err != nil {
		return fmt.Errorf("hetzner client got error %w", err)
	} else {
		nm.hetznerVMs = getTypeInfosHetzner(servers)
	}
	return nil
}

// cacheAws function uses aws-sdk-go-v2 module to query supported VMs and their info. If the query is successful, the VM info is saved in cache.
func (nm *NodeManager) cacheAws(np *pb.NodePool) error {
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
		return fmt.Errorf("AWS config got error : %w", err)
	}
	client := ec2.NewFromConfig(cfg)
	maxResults := int32(defaultMaxResults)
	var token *string
	// Use while loop to support paging.
	for {
		if res, err := client.DescribeInstanceTypes(context.Background(), &ec2.DescribeInstanceTypesInput{MaxResults: &maxResults, NextToken: token}); err != nil {
			return fmt.Errorf("AWS client got error : %w", err)
		} else {
			nm.awsVMs = mergeMaps(getTypeInfosAws(res.InstanceTypes), nm.awsVMs)
			// Check if there are any more results to query.
			token = res.NextToken
			if res.NextToken == nil {
				break
			}
		}
	}
	return nil
}

// cacheGcp function uses google go module to query supported VMs and their info. If the query is successful, the VM info is saved in cache.
func (nm *NodeManager) cacheGcp(np *pb.NodePool) error {
	// Create client and create cache
	computeService, err := compute.NewMachineTypesRESTClient(context.Background(), option.WithCredentialsJSON([]byte(np.Provider.Credentials)))
	if err != nil {
		return fmt.Errorf("GCP client got error : %w", err)
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
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return fmt.Errorf("GCP client got error while looping: %w", err)
		}
		machineTypes = append(machineTypes, mt)
	}
	nm.gcpVMs = mergeMaps(getTypeInfosGcp(machineTypes), nm.gcpVMs)
	return nil
}

// cacheOci function uses oci-go-sdk module to query supported shapes and their info. If the query is successful, the shape info is saved in cache.
func (nm *NodeManager) cacheOci(np *pb.NodePool) error {
	conf := common.NewRawConfigurationProvider(np.Provider.OciTenancyOcid, np.Provider.OciUserOcid, np.Region, np.Provider.OciFingerprint, np.Provider.Credentials, nil)
	client, err := core.NewComputeClientWithConfigurationProvider(conf)
	if err != nil {
		return fmt.Errorf("OCI client got error : %w", err)
	}
	maxResults := defaultMaxResults
	req := core.ListShapesRequest{
		CompartmentId: &np.Provider.OciCompartmentOcid,
		Limit:         &maxResults,
	}
	for {
		r, err := client.ListShapes(context.Background(), req)
		if err != nil {
			return fmt.Errorf("OCI client got error : %w", err)
		}
		if r.Items == nil || len(r.Items) == 0 {
			return fmt.Errorf("OCI client got empty response")
		}
		nm.ociVMs = mergeMaps(getTypeInfosOci(r.Items), nm.ociVMs)
		if r.OpcNextPage != nil {
			req.Page = r.OpcNextPage
		} else {
			break
		}
	}
	return nil
}

// cacheOci function uses oci-go-sdk module to query supported VMs and their info. If the query is successful, the VM info is saved in cache.
func (nm *NodeManager) cacheAzure(np *pb.NodePool) error {
	cred, err := azidentity.NewClientSecretCredential(np.Provider.AzureTenantId, np.Provider.AzureClientId, np.Provider.Credentials, nil)
	if err != nil {
		return fmt.Errorf("azure client got error : %w", err)
	}

	client, err := armcompute.NewVirtualMachineSizesClient(np.Provider.AzureSubscriptionId, cred, nil)
	if err != nil {
		return fmt.Errorf("azure client got error : %w", err)
	}
	location := strings.ToLower(strings.ReplaceAll(np.Region, " ", ""))
	pager := client.NewListPager(location, nil)

	for pager.More() {
		nextResult, err := pager.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("azure client got error : %w", err)
		}
		nm.azureVMs = mergeMaps(getTypeInfosAzure(nextResult.Value), nm.azureVMs)
	}
	return nil
}
