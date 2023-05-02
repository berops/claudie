package terraformer

import (
	"testing"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
)

var desiredState = &pb.Project{
	Name: "TestProjectName",
	Clusters: []*pb.K8Scluster{
		{
			ClusterInfo: &pb.ClusterInfo{
				Name:       "cluster1",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
				NodePools: []*pb.NodePool{
					{
						Name:            "NodePoolName1-Master",
						Region:          "Autralia",
						ServerType:      "cpx11",
						Image:           "ubuntu-20.04",
						StorageDiskSize: 20,
						Zone:            "example zone",
						Count:           1,
						Provider: &pb.Provider{
							SpecName:          "hetzner-1",
							Credentials:       "api-token",
							CloudProviderName: "hetzner",
						},
					},
					{
						Name:            "NodePoolName1-Worker",
						Region:          "Autralia",
						ServerType:      "cpx11",
						Image:           "ubuntu-20.04",
						StorageDiskSize: 20,
						Zone:            "example zone",
						Count:           2,
						Provider: &pb.Provider{
							SpecName:          "hetzner-1",
							Credentials:       "api-token",
							CloudProviderName: "hetzner",
						},
					},
					{
						Name:            "NodePoolName2-Master",
						Region:          "Autralia",
						ServerType:      "e2-small",
						Image:           "ubuntu-os-cloud/ubuntu-2004-focal-v20220610",
						StorageDiskSize: 20,
						Zone:            "example zone",
						Count:           1,
						Provider: &pb.Provider{
							SpecName:          "gcp-1",
							Credentials:       "sak.json",
							CloudProviderName: "gcp",
						},
					},
					{
						Name:            "NodePoolName2-Worker",
						Region:          "Autralia",
						ServerType:      "e2-small",
						Image:           "ubuntu-os-cloud/ubuntu-2004-focal-v20220610",
						StorageDiskSize: 20,
						Zone:            "example zone",
						Count:           2,
						Provider: &pb.Provider{
							SpecName:          "gcp-1",
							Credentials:       "sak.json",
							CloudProviderName: "gcp",
						},
					},
				},
			},
			Kubernetes: "19.0",
			Network:    "192.168.2.0/24",
			Kubeconfig: "ExampleKubeConfig",
		},
		{
			ClusterInfo: &pb.ClusterInfo{
				Name:       "cluster2",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
				NodePools: []*pb.NodePool{
					{
						Name:            "NodePoolName3-Master",
						Region:          "Autralia",
						ServerType:      "cpx11",
						Image:           "ubuntu-20.04",
						StorageDiskSize: 20,
						Zone:            "example zone",
						Count:           1,
						Provider: &pb.Provider{
							SpecName:          "hetzner-1",
							Credentials:       "api-token",
							CloudProviderName: "hetzner",
						},
					},
					{
						Name:            "NodePoolName3-Worker",
						Region:          "Autralia",
						ServerType:      "cpx11",
						Image:           "ubuntu-20.04",
						StorageDiskSize: 20,
						Zone:            "example zone",
						Count:           2,
						Provider: &pb.Provider{
							SpecName:          "hetzner-1",
							Credentials:       "api-token",
							CloudProviderName: "hetzner",
						},
					},
					{
						Name:            "NodePoolName4-Master",
						Region:          "Autralia",
						ServerType:      "e2-small",
						Image:           "ubuntu-os-cloud/ubuntu-2004-focal-v20220610",
						StorageDiskSize: 20,
						Zone:            "example zone",
						Count:           1,
						Provider: &pb.Provider{
							SpecName:          "gcp-1",
							Credentials:       "sak.json",
							CloudProviderName: "gcp",
						},
					},
					{
						Name:            "NodePoolName4-Worker",
						Region:          "Autralia",
						ServerType:      "e2-small",
						Image:           "ubuntu-os-cloud/ubuntu-2004-focal-v20220610",
						StorageDiskSize: 20,
						Zone:            "example zone",
						Count:           2,
						Provider: &pb.Provider{
							SpecName:          "gcp-1",
							Credentials:       "sak.json",
							CloudProviderName: "gcp",
						},
					},
				},
			},

			Kubernetes: "20.1",
			Network:    "192.168.2.0/24",
			Kubeconfig: "ExampleKubeConfig",
		},
	},
	LoadBalancerClusters: []*pb.LBcluster{
		{
			ClusterInfo: &pb.ClusterInfo{
				Name:       "cluster1-api-server",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
				NodePools: []*pb.NodePool{
					{
						Name:            "NodePoolName-LB",
						Region:          "Autralia",
						ServerType:      "e2-small",
						Image:           "ubuntu-os-cloud/ubuntu-2004-focal-v20220610",
						StorageDiskSize: 20,
						Zone:            "example zone",
						Count:           2,
						Provider: &pb.Provider{
							SpecName:          "gcp-1",
							Credentials:       "sak.json",
							CloudProviderName: "gcp",
						},
					},
				},
			},
			Roles: []*pb.Role{
				{
					Name:       "api-server-lb",
					Port:       6443,
					TargetPort: 6443,
					Target:     pb.Target_k8sControlPlane,
				},
			},
			Dns: &pb.DNS{
				DnsZone:  "lb-zone",
				Hostname: "www.test.io",
				Provider: &pb.Provider{
					SpecName:          "gcp-1",
					Credentials:       "sak.json",
					CloudProviderName: "gcp",
				},
			},
		},
	},
}

func TestBuildInfrastructure(t *testing.T) {
	// Create connection to Terraformer
	cc, err := utils.GrpcDialWithInsecure("terraformer", envs.TerraformerURL)
	if err != nil {
		log.Fatal().Err(err)
	}
	defer func() {
		err := cc.Close()
		require.NoError(t, err)
	}()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)

	res, err := BuildInfrastructure(c, &pb.BuildInfrastructureRequest{
		Current:     nil,
		Desired:     desiredState.Clusters[0],
		DesiredLbs:  desiredState.LoadBalancerClusters,
		ProjectName: desiredState.Name,
	})
	require.NoError(t, err)
	t.Log("Terraformer response: ", res)

	// Print just public ip addresses
	t.Log(res.GetCurrent().GetClusterInfo().GetName())
	for i, nodepool := range res.GetCurrent().GetClusterInfo().GetNodePools() {
		for k, node := range nodepool.Nodes {
			t.Log(i+k, node.GetPublic())
		}
	}
}
