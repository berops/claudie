package clusterBuilder

import (
	"fmt"
	"testing"

	"github.com/Berops/claudie/internal/templateUtils"
	"github.com/Berops/claudie/proto/pb"
	"github.com/stretchr/testify/require"
)

var desiredState *pb.Project = &pb.Project{
	Name: "TestProjectName",
	Clusters: []*pb.K8Scluster{
		{
			ClusterInfo: &pb.ClusterInfo{
				Name:       "cluster1",
				PublicKey:  "public-key",
				PrivateKey: "private-key",
				NodePools: []*pb.NodePool{
					{
						Name:       "NodePoolName1-Master",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      1,
						Provider: &pb.Provider{
							SpecName:          "hetzner-1",
							Credentials:       "api-token",
							CloudProviderName: "hetzner",
						},
					},
					{
						Name:       "NodePoolName1-Worker",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      2,
						Provider: &pb.Provider{
							SpecName:          "hetzner-1",
							Credentials:       "api-token",
							CloudProviderName: "hetzner",
						},
					},
					{
						Name:       "NodePoolName2-Master",
						Region:     "Autralia",
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-focal-v20220610",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      1,
						Provider: &pb.Provider{
							SpecName:          "gcp-1",
							Credentials:       "sak.json",
							CloudProviderName: "gcp",
						},
					},
					{
						Name:       "NodePoolName2-Worker",
						Region:     "Autralia",
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-focal-v20220610",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      2,
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
						Name:       "NodePoolName3-Master",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      1,
						Provider: &pb.Provider{
							SpecName:          "hetzner-1",
							Credentials:       "api-token",
							CloudProviderName: "hetzner",
						},
					},
					{
						Name:       "NodePoolName3-Worker",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      2,
						Provider: &pb.Provider{
							SpecName:          "hetzner-1",
							Credentials:       "api-token",
							CloudProviderName: "hetzner",
						},
					},
					{
						Name:       "NodePoolName4-Master",
						Region:     "Autralia",
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-focal-v20220610",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      1,
						Provider: &pb.Provider{
							SpecName:          "gcp-1",
							Credentials:       "sak.json",
							CloudProviderName: "gcp",
						},
					},
					{
						Name:       "NodePoolName4-Worker",
						Region:     "Autralia",
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-focal-v20220610",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      2,
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
						Name:       "NodePoolName-LB",
						Region:     "Autralia",
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-focal-v20220610",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      2,
						Provider: &pb.Provider{
							SpecName:          "gcp-1",
							Credentials:       "sak.json",
							CloudProviderName: "gcp",
						},
						Nodes: []*pb.Node{
							{
								Name:     "testName1",
								Private:  "1.1.1.1",
								Public:   "34.0.9.1",
								NodeType: pb.NodeType_worker,
							},
							{
								Name:     "testName2",
								Private:  "1.1.1.1",
								Public:   "34.0.9.2",
								NodeType: pb.NodeType_worker,
							},
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

var jsonData = "{\"compute\":{\"test-cluster-compute1\":\"0.0.0.65\",\n\"test-cluster-compute2\":\"0.0.0.512\"},\n\"control\":{\"test-cluster-control1\":\"0.0.0.72\",\n\"test-cluster-control2\":\"0.0.0.65\"}}"

var ociNp = &pb.NodePool{
	Name:       "test-np",
	Region:     "TEST_REGION",
	ServerType: "TEST_SERVER_TYPE",
	Image:      "TEST_IMAGE",
	DiskSize:   50,
	Zone:       "TEST_ZONE",
	Count:      3,
	Nodes:      []*pb.Node{},
	IsControl:  true,
	Provider: &pb.Provider{
		CloudProviderName: "oci",
		Credentials:       "TEST_PRIVATE_KEY",
		OciFingerprint:    "TEST_FINGERPRINT",
		TenancyOcid:       "TEST_TENANCY",
		UserOcid:          "TEST_USER",
		OciCompartmentId:  "TEST_COMPARTMENT",
	},
}

func TestReadOutput(t *testing.T) {
	out, err := readIPs(jsonData)
	if err == nil {
		t.Log(out.IPs)
	}
	require.NoError(t, err)
}

func TestFillNodes(t *testing.T) {
	out, err := readIPs(jsonData)
	if err == nil {
		var m = &pb.NodePool{}
		fillNodes(&out, m, desiredState.Clusters[0].ClusterInfo.NodePools[0].Nodes)
		fmt.Println(m)
	}
	require.NoError(t, err)
}

func TestGenerateTf(t *testing.T) {
	templateLoader := templateUtils.TemplateLoader{Directory: "../../templates"}
	template := templateUtils.Templates{Directory: "."}
	tpl, err := templateLoader.LoadTemplate("oci.tpl")
	require.NoError(t, err)
	err = template.Generate(tpl, "oci-test.tf", &NodepoolsData{ClusterName: "TEST_NAME", ClusterHash: "TEST_HASH", NodePools: []*pb.NodePool{ociNp}})
	require.NoError(t, err)
}
