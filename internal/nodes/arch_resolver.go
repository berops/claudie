package nodes

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"net/http"
	"regexp"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/option"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/berops/claudie/proto/pb"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/rs/zerolog/log"
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
	Arch(node *pb.NodePool) (Arch, error)
}

// DynamicNodePoolResolver will resolve architecture for a dynamic nodepool.
type DynamicNodePoolResolver struct {
	cache map[string]Arch
}

func NewDynamicNodePoolResolver(init []*pb.DynamicNodePool) (*DynamicNodePoolResolver, error) {
	r := &DynamicNodePoolResolver{cache: make(map[string]Arch)}

	for _, np := range init {
		arch, err := r.resolve(np)
		if err != nil {
			return nil, err
		}
		r.cache[fmt.Sprintf("%s-%s", np.GetProvider().GetCloudProviderName(), np.GetServerType())] = arch
	}

	return r, nil
}

func (r *DynamicNodePoolResolver) Arch(np *pb.NodePool) (Arch, error) {
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

func (r *DynamicNodePoolResolver) resolve(np *pb.DynamicNodePool) (Arch, error) {
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
	default:
		return "", fmt.Errorf("%q not supported", np.GetProvider().GetCloudProviderName())
	}
}

func resolveAzure(np *pb.DynamicNodePool) (Arch, error) {
	cred, err := azidentity.NewClientSecretCredential(np.Provider.AzureTenantId, np.Provider.AzureClientId, np.Provider.Credentials, nil)
	if err != nil {
		return "", fmt.Errorf("azure client got error : %w", err)
	}

	imgClient, err := armcompute.NewVirtualMachineImagesClient(np.Provider.AzureSubscriptionId, cred, nil)
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

func resolveOci(np *pb.DynamicNodePool) (Arch, error) {
	// OCI sdk doesn't have a way to retrieve the Arch thus we default to a regex match
	// TODO: find a better way to handle this case.
	arch := Amd64
	ok, err := regexp.MatchString("^.+\\.A1\\..+$", np.Image)
	if err != nil {
		return "", fmt.Errorf("%w %w", ErrFailedToResolveArch, err)
	}
	if ok {
		arch = Arm64
	}

	return Arch(arch), nil
}

func resolveGcp(np *pb.DynamicNodePool) (Arch, error) {
	imgClient, err := compute.NewImagesRESTClient(context.Background(), option.WithCredentialsJSON([]byte(np.Provider.Credentials)))
	if err != nil {
		return "", fmt.Errorf("failed to create GCP client error : %w", err)
	}
	defer func() {
		if err := imgClient.Close(); err != nil {
			log.Err(err).Msgf("Failed to close GCP client")
		}
	}()

	imgInfo, err := imgClient.Get(context.Background(), &computepb.GetImageRequest{
		Project: np.Provider.GcpProject,
		Image:   np.Image,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("error retrieving img info: %w", err)
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

func resolveHetzner(np *pb.DynamicNodePool) (Arch, error) {
	hc := hcloud.NewClient(hcloud.WithToken(np.Provider.Credentials), hcloud.WithHTTPClient(http.DefaultClient))

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

func resolveAws(np *pb.DynamicNodePool) (Arch, error) {
	credFunc := func(lo *config.LoadOptions) error {
		lo.Credentials = aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     np.Provider.AwsAccessKey,
				SecretAccessKey: np.Provider.Credentials,
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
	pageSize := int32(10)

	for {
		res, err := client.DescribeInstanceTypes(context.Background(), &ec2.DescribeInstanceTypesInput{
			MaxResults: &pageSize,
			NextToken:  token,
			InstanceTypes: []types.InstanceType{
				types.InstanceType(np.ServerType),
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to list instance types for aws: %w", err)
		}

		// debug.
		fmt.Printf("--- %#v\n", res.InstanceTypes)
		for _, i := range res.InstanceTypes {
			fmt.Printf("--- %#v\n", i.ProcessorInfo.SupportedArchitectures)
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
