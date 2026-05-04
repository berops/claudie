package node_manager

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/berops/claudie/internal/generics"
	"github.com/berops/claudie/proto/pb/spec"
	egoscale "github.com/exoscale/egoscale/v3"
	exocredentials "github.com/exoscale/egoscale/v3/credentials"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	defaultMaxResults       = 30
	cloudRiftAPIVersion     = "2025-06-10"
	cloudRiftAPIBaseURL     = "https://api.cloudrift.ai/api/v1"
	cloudRiftRequestTimeout = 30 * time.Second

	verdaAPIBaseURL     = "https://api.verda.com/v1"
	verdaTokenPath      = "/oauth2/token"
	verdaScope          = "cloud-api-v1"
	verdaRequestTimeout = 30 * time.Second
)

// cacheHetzner function uses hcloud-go module to query supported servers and their info. If the query is successful, the server info is saved in cache.
func (nm *NodeManager) cacheHetzner(np *spec.DynamicNodePool) error {
	// Create client and create cache.
	hc := hcloud.NewClient(hcloud.WithToken(np.Provider.GetHetzner().Token), hcloud.WithHTTPClient(http.DefaultClient))
	servers, err := hc.ServerType.All(context.Background())
	if err != nil {
		return fmt.Errorf("hetzner client got error %w", err)
	}
	nm.hetznerVMs = getTypeInfoHetzner(servers)
	return nil
}

// cacheAws function uses aws-sdk-go-v2 module to query supported VMs and their info. If the query is successful, the VM info is saved in cache.
func (nm *NodeManager) cacheAws(np *spec.DynamicNodePool) error {
	// Define option function to set static credentials
	credFunc := func(lo *config.LoadOptions) error {
		lo.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: np.Provider.GetAws().AccessKey, SecretAccessKey: np.Provider.GetAws().SecretKey}, nil
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
		res, err := client.DescribeInstanceTypes(context.Background(), &ec2.DescribeInstanceTypesInput{MaxResults: &maxResults, NextToken: token})
		if err != nil {
			return fmt.Errorf("AWS client got error : %w", err)
		}
		nm.awsVMs = generics.MergeMaps(getTypeInfoAws(res.InstanceTypes), nm.awsVMs)
		// Check if there are any more results to query.
		token = res.NextToken
		if res.NextToken == nil {
			break
		}
	}
	return nil
}

// cacheGcp function uses google go module to query supported VMs and their info. If the query is successful, the VM info is saved in cache.
func (nm *NodeManager) cacheGcp(np *spec.DynamicNodePool) error {
	creds, err := google.CredentialsFromJSON(
		context.Background(),
		[]byte(np.Provider.GetGcp().Key),
		compute.DefaultAuthScopes()...,
	)
	if err != nil {
		return fmt.Errorf("failed to parse GCP credentials: %w", err)
	}

	computeService, err := compute.NewMachineTypesRESTClient(context.Background(), option.WithCredentials(creds))
	if err != nil {
		return fmt.Errorf("GCP client got error : %w", err)
	}
	defer func() {
		if err := computeService.Close(); err != nil {
			log.Err(err).Msgf("Failed to close GCP client")
		}
	}()

	// Use project-scoped aggregated list to avoid requiring a specific zone.
	maxResults := uint32(defaultMaxResults)
	retPartialSuccess := true
	req := &computepb.AggregatedListMachineTypesRequest{
		Project:              np.Provider.GetGcp().Project,
		MaxResults:           &maxResults,
		ReturnPartialSuccess: &retPartialSuccess,
	}
	it := computeService.AggregatedList(context.Background(), req)
	machineTypes := make([]*computepb.MachineType, 0)
	for {
		pair, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return fmt.Errorf("GCP client got error while looping: %w", err)
		}
		machineTypes = append(machineTypes, pair.Value.GetMachineTypes()...)
	}
	nm.gcpVMs = generics.MergeMaps(getTypeInfoGcp(machineTypes), nm.gcpVMs)

	return nil
}

// cacheOci function uses oci-go-sdk module to query supported shapes and their info. If the query is successful, the shape info is saved in cache.
func (nm *NodeManager) cacheOci(np *spec.DynamicNodePool) error {
	conf := common.NewRawConfigurationProvider(np.Provider.GetOci().TenancyOCID, np.Provider.GetOci().UserOCID, np.Region, np.Provider.GetOci().KeyFingerprint, np.Provider.GetOci().PrivateKey, nil)
	client, err := core.NewComputeClientWithConfigurationProvider(conf)
	if err != nil {
		return fmt.Errorf("OCI client got error : %w", err)
	}
	maxResults := defaultMaxResults
	req := core.ListShapesRequest{
		CompartmentId: &np.Provider.GetOci().CompartmentOCID,
		Limit:         &maxResults,
	}
	for {
		r, err := client.ListShapes(context.Background(), req)
		if err != nil {
			return fmt.Errorf("OCI client got error : %w", err)
		}
		if len(r.Items) == 0 {
			return fmt.Errorf("OCI client got empty response")
		}
		nm.ociVMs = generics.MergeMaps(getTypeInfoOci(r.Items), nm.ociVMs)
		if r.OpcNextPage != nil {
			req.Page = r.OpcNextPage
		} else {
			break
		}
	}

	return nil
}

// cacheAzure function uses azure-sdk-for-go module to query supported VMs and their info. If the query is successful, the VM info is saved in cache.
func (nm *NodeManager) cacheAzure(np *spec.DynamicNodePool) error {
	cred, err := azidentity.NewClientSecretCredential(np.Provider.GetAzure().TenantID, np.Provider.GetAzure().ClientID, np.Provider.GetAzure().ClientSecret, nil)
	if err != nil {
		return fmt.Errorf("azure client got error : %w", err)
	}

	client, err := armcompute.NewVirtualMachineSizesClient(np.Provider.GetAzure().SubscriptionID, cred, nil)
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
		nm.azureVMs = generics.MergeMaps(getTypeInfoAzure(nextResult.Value), nm.azureVMs)
	}

	return nil
}

func (nm *NodeManager) cacheOpenstack(np *spec.DynamicNodePool) error {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint:            np.Provider.GetOpenstack().AuthURL,
		ApplicationCredentialID:     np.Provider.GetOpenstack().ApplicationCredentialID,
		ApplicationCredentialSecret: np.Provider.GetOpenstack().ApplicationCredentialSecret,
	}

	authClient, err := openstack.AuthenticatedClient(context.Background(), authOpts)
	if err != nil {
		return fmt.Errorf("openstack authentication got error : %w", err)
	}

	computeClient, err := openstack.NewComputeV2(authClient, gophercloud.EndpointOpts{
		Region: np.Region,
	})
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}

	allPages, err := flavors.ListDetail(computeClient, flavors.ListOpts{}).AllPages(context.Background())
	if err != nil {
		return fmt.Errorf("openstack client got error : %w", err)
	}

	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return fmt.Errorf("openstack client got error : %w", err)
	}
	nm.openstackVMs = getTypeInfoOpenstack(allFlavors)
	return nil
}

func (nm *NodeManager) cacheExoscale(np *spec.DynamicNodePool) error {
	creds := exocredentials.NewStaticCredentials(np.Provider.GetExoscale().ApiKey, np.Provider.GetExoscale().ApiSecret)
	client, err := egoscale.NewClient(creds)
	if err != nil {
		return fmt.Errorf("exoscale client got error: %w", err)
	}

	resp, err := client.ListInstanceTypes(context.Background())
	if err != nil {
		return fmt.Errorf("exoscale client got error: %w", err)
	}

	nm.exoscaleVMs = getTypeInfoExoscale(resp.InstanceTypes)
	return nil
}

func (nm *NodeManager) cacheCloudRift(np *spec.DynamicNodePool) error {
	token := np.Provider.GetCloudrift().Token

	ctx, cancel := context.WithTimeout(context.Background(), cloudRiftRequestTimeout)
	defer cancel()

	reqBody := strings.NewReader(`{"version":"` + cloudRiftAPIVersion + `","data":{"selector":"All"}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cloudRiftAPIBaseURL+"/instance-types/list", reqBody)
	if err != nil {
		return fmt.Errorf("cloudrift client error: %w", err)
	}
	req.Header.Set("X-API-KEY", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudrift client error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("cloudrift API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result cloudRiftInstanceTypesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("cloudrift client error parsing response: %w", err)
	}

	nm.cloudriftVMs = getTypeInfoCloudRift(result.Data.InstanceTypes)
	return nil
}

// cacheVerda fetches the Verda Cloud instance-type catalog using OAuth2 client_credentials.
// The OpenTofu provider handles auth itself for terraform-side resource calls; this is the
// only place in Claudie that talks to Verda's REST API directly.
func (nm *NodeManager) cacheVerda(np *spec.DynamicNodePool) error {
	v := np.Provider.GetVerda()
	baseURL := verdaAPIBaseURL
	if v.GetBaseUrl() != "" {
		baseURL = strings.TrimRight(v.GetBaseUrl(), "/")
	}

	cfg := clientcredentials.Config{
		ClientID:     v.GetClientId(),
		ClientSecret: v.GetClientSecret(),
		TokenURL:     baseURL + verdaTokenPath,
		Scopes:       []string{verdaScope},
	}

	ctx, cancel := context.WithTimeout(context.Background(), verdaRequestTimeout)
	defer cancel()

	httpClient := cfg.Client(ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/instance-types", nil)
	if err != nil {
		return fmt.Errorf("verda client error: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("verda client error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("verda API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result verdaInstanceTypesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("verda client error parsing response: %w", err)
	}

	nm.verdaVMs = getTypeInfoVerda(result)
	return nil
}
