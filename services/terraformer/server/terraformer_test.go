package main

import (
	"testing"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/terraformer/server/clusterBuilder"
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
							SpecName:    "hetzner",
							Credentials: "api-token",
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
							SpecName:    "hetzner",
							Credentials: "api-token",
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
							SpecName:    "gcp",
							Credentials: "sak.json",
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
							SpecName:    "gcp",
							Credentials: "sak.json",
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
							SpecName:    "hetzner",
							Credentials: "api-token",
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
							SpecName:    "hetzner",
							Credentials: "api-token",
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
							SpecName:    "gcp",
							Credentials: "sak.json",
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
							SpecName:    "gcp",
							Credentials: "sak.json",
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
							SpecName:    "gcp",
							Credentials: "sak.json",
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
					SpecName:    "gcp",
					Credentials: "keyfile.json",
				},
			},
		},
	},
}

var testState *pb.Project = &pb.Project{
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
						Count:      3,
						Provider: &pb.Provider{
							SpecName:    "hetzner",
							Credentials: "api-token",
						},
					},
					{
						Name:       "NodePoolName1-Worker",
						Region:     "Autralia",
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Count:      3,
						Provider: &pb.Provider{
							SpecName:    "hetzner",
							Credentials: "api-token",
						},
					},
				},
			},
			Kubernetes: "20.1",
			Network:    "192.168.2.0/24",
			Kubeconfig: "ExampleKubeConfig",
		},
	},
}

func TestBuildInfrastructure(t *testing.T) {
	clusterBuilder := clusterBuilder.ClusterBuilder{DesiredInfo: testState.Clusters[0].ClusterInfo, CurrentInfo: nil, ProjectName: "GO TEST", ClusterType: pb.ClusterType_K8s}
	err := clusterBuilder.CreateNodepools()
	require.NoError(t, err)
}

func TestBuildLBNodepools(t *testing.T) {
	clusterBuilder := clusterBuilder.ClusterBuilder{DesiredInfo: desiredState.LoadBalancerClusters[0].ClusterInfo, CurrentInfo: nil, ProjectName: "GO TEST", ClusterType: pb.ClusterType_LB}
	err := clusterBuilder.CreateNodepools()
	require.NoError(t, err)
}
