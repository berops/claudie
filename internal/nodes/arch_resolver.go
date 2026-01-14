package nodes

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
)

var (
	// ErrFailedToResolveArch is returned when resolving the architecture failed.
	ErrFailedToResolveArch = errors.New("failed to resolve architecture for nodepool")
)

type Arch string

const (
	Amd64 = "amd64"
	Arm64 = "arm64"
)

// ArchResolver resolves the architecture of a nodepool.
type ArchResolver interface {
	Arch(node *spec.NodePool) (Arch, error)
}

// DynamicNodePoolResolver will resolve architecture for a dynamic nodepool.
type DynamicNodePoolResolver struct {
	cache map[string]Arch
}

func NewDynamicNodePoolResolver() *DynamicNodePoolResolver {
	return &DynamicNodePoolResolver{
		cache: make(map[string]Arch),
	}
}

func (r *DynamicNodePoolResolver) Arch(np *spec.NodePool) (Arch, error) {
	if np.GetDynamicNodePool() == nil {
		return "", ErrFailedToResolveArch
	}

	dyn := np.GetDynamicNodePool()
	key := fmt.Sprintf("%s-%s", dyn.GetProvider().GetCloudProviderName(), dyn.GetServerType())
	if res, ok := r.cache[key]; ok {
		return res, nil
	}

	arch, err := r.resolve(dyn)
	if err != nil {
		return "", err
	}

	r.cache[key] = arch
	return arch, nil
}

func (r *DynamicNodePoolResolver) resolve(np *spec.DynamicNodePool) (Arch, error) {
	switch np.GetProvider().GetCloudProviderName() {
	case "hetzner":
		return resolveHetzner(np)
	case "aws":
		return resolveAws(np)
	case "gcp":
		return resolveGcp(np)
	case "oci":
		return resolveOci(np)
	case "azure":
		return resolveAzure(np)
	case "genesiscloud":
		return resolveGenesisCloud(np)
	case "openstack":
		return resolveOpenstack(np)
	default:
		return "", fmt.Errorf("%q not supported", np.GetProvider().GetCloudProviderName())
	}
}

func resolveOpenstack(np *spec.DynamicNodePool) (Arch, error) {
	// As of October 3, 2025, there is no way to determine the OpenStack
	// architecture resolution based on the image or server type/flavor.
	return Amd64, nil
}

func resolveGenesisCloud(np *spec.DynamicNodePool) (Arch, error) {
	// As of now (15. oct 2024) genesiscloud currently only has x64 cpus.
	return Amd64, nil
}

func resolveAzure(np *spec.DynamicNodePool) (Arch, error) {
	cred, err := azidentity.NewClientSecretCredential(np.Provider.GetAzure().TenantID, np.Provider.GetAzure().ClientID, np.Provider.GetAzure().ClientSecret, nil)
	if err != nil {
		return "", fmt.Errorf("azure client got error : %w", err)
	}

	imgClient, err := armcompute.NewVirtualMachineImagesClient(np.Provider.GetAzure().SubscriptionID, cred, nil)
	if err != nil {
		return "", fmt.Errorf("azure client errored: %w", err)
	}

	imgParts := strings.Split(np.Image, ":")
	publisher := imgParts[0]
	offer := imgParts[1]
	sku := imgParts[2]
	version := imgParts[3]

	location := strings.ToLower(strings.ReplaceAll(np.Region, " ", ""))
	resp, err := imgClient.Get(context.Background(), location, publisher, offer, sku, version, nil)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve azure image info: %w", err)
	}

	arch := Amd64
	if *resp.Properties.Architecture == armcompute.ArchitectureTypesArm64 {
		arch = Arm64
	}

	return Arch(arch), nil
}

func resolveOci(np *spec.DynamicNodePool) (Arch, error) {
	// OCI sdk doesn't have a way to retrieve the Arch thus we default to a regex match
	// TODO: find a better way to handle this case.
	arch := Amd64
	ok, err := regexp.MatchString("^.+\\.A1\\..+$", np.ServerType)
	if err != nil {
		return "", fmt.Errorf("%w %w", ErrFailedToResolveArch, err)
	}
	if ok {
		arch = Arm64
	}

	return Arch(arch), nil
}

func resolveGcp(np *spec.DynamicNodePool) (Arch, error) {
	imgClient, err := compute.NewImagesRESTClient(context.Background(), option.WithCredentialsJSON([]byte(np.Provider.GetGcp().Key)))
	if err != nil {
		return "", fmt.Errorf("failed to create GCP client error : %w", err)
	}
	defer func() {
		if err := imgClient.Close(); err != nil {
			log.Err(err).Msgf("Failed to close GCP client")
		}
	}()

	imgInfo, err := imgClient.Get(context.Background(), &computepb.GetImageRequest{
		Project: np.Provider.GetGcp().Project,
		Image:   np.Image,
	})
	if err != nil {
		log.Debug().Msgf("error retrieving img info: %s", err)
		log.Debug().Msgf("matching against server type to determine the architecture")

		// if the img is not recognized by gcloud we have a situation similar to OCI as to where
		// there is not way at this time to determine the architecture of the machinte type via the sdk.
		// thus we default to a regex match against known ARM server types.
		arch := Amd64
		ok, err := regexp.MatchString("^t2a\\-.+$", np.ServerType)
		if err != nil {
			return "", fmt.Errorf("%w %w", ErrFailedToResolveArch, err)
		}
		if ok {
			arch = Arm64
		}
		return Arch(arch), nil
	}

	if imgInfo == nil {
		return "", fmt.Errorf("%w - failed to find img %s for server: %s", ErrFailedToResolveArch, np.Image, np.ServerType)
	}

	arch := Amd64
	if *imgInfo.Architecture == string(computepb.Image_ARM64) {
		arch = Arm64
	}

	return Arch(arch), nil
}

func resolveHetzner(np *spec.DynamicNodePool) (Arch, error) {
	hc := hcloud.NewClient(hcloud.WithToken(np.Provider.GetHetzner().Token), hcloud.WithHTTPClient(http.DefaultClient))

	typ, _, err := hc.ServerType.GetByName(context.Background(), np.ServerType)
	if err != nil {
		return "", fmt.Errorf("hetzner client error: %w", err)
	}

	if typ == nil {
		return "", fmt.Errorf("%w - failed to find server type: %s", ErrFailedToResolveArch, np.ServerType)
	}

	if typ.Architecture == hcloud.ArchitectureARM {
		return Arm64, nil
	}
	return Amd64, nil
}

func resolveAws(np *spec.DynamicNodePool) (Arch, error) {
	credFunc := func(lo *config.LoadOptions) error {
		lo.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     np.Provider.GetAws().AccessKey,
				SecretAccessKey: np.Provider.GetAws().SecretKey,
			}, nil
		})
		return nil
	}

	httpClient := awshttp.NewBuildableClient().WithTransportOptions(func(tr *http.Transport) {
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		}
		tr.TLSClientConfig.MinVersion = tls.VersionTLS12
	})

	cfg, err := config.LoadDefaultConfig(context.Background(), credFunc, config.WithHTTPClient(httpClient), config.WithRegion(np.Region))
	if err != nil {
		return "", fmt.Errorf("AWS config error : %w", err)
	}

	client := ec2.NewFromConfig(cfg)

	var arch Arch
	var token *string

	for {
		res, err := client.DescribeInstanceTypes(context.Background(), &ec2.DescribeInstanceTypesInput{
			NextToken: token,
			InstanceTypes: []types.InstanceType{
				types.InstanceType(np.ServerType),
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to list instance types for aws: %w", err)
		}

		if len(res.InstanceTypes) > 0 {
			instance := res.InstanceTypes[0]
			for _, architecture := range instance.ProcessorInfo.SupportedArchitectures {
				if architecture == types.ArchitectureTypeArm64 || architecture == types.ArchitectureTypeArm64Mac {
					arch = Arm64
				} else {
					arch = Amd64
				}
			}
		}

		token = res.NextToken
		if res.NextToken == nil {
			break
		}
	}

	if arch == "" {
		return "", fmt.Errorf("%w failed to resolve architecture for server type %s", ErrFailedToResolveArch, np.ServerType)
	}

	return arch, nil
}
