package outboundAdapters

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/proto/pb"
)

var (
	desiredState *pb.Project = &pb.Project{
		Name: "TestProjectName",
		Clusters: []*pb.K8Scluster{
			{
				ClusterInfo: &pb.ClusterInfo{
					Name: "cluster1",
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
					Name: "cluster2",
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
					Name: "cluster1-api-server",
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
						CloudProviderName: "gcp",
					},
				},
			},
		},
	}
)

func TestSaveConfig(t *testing.T) {
	mongoDBConnector := NewMongoDBConnector(envs.DatabaseURL)
	err := mongoDBConnector.Connect()
	require.NoError(t, err)
	err = mongoDBConnector.Init()
	require.NoError(t, err)
	defer mongoDBConnector.Disconnect()
	conf := &pb.Config{DesiredState: desiredState, Name: "test-pb-config"}
	err = mongoDBConnector.SaveConfig(conf)
	require.NoError(t, err)
	fmt.Println("Config id: " + conf.Id)
	require.NotEmpty(t, conf.Id)
	err = mongoDBConnector.DeleteConfig(conf.Name, pb.IdType_NAME)
	require.NoError(t, err)
}

func TestUpdateTTL(t *testing.T) {
	mongoDBConnector := NewMongoDBConnector(envs.DatabaseURL)
	err := mongoDBConnector.Connect()
	require.NoError(t, err)
	err = mongoDBConnector.Init()
	require.NoError(t, err)
	defer mongoDBConnector.Disconnect()
	conf := &pb.Config{DesiredState: desiredState, Name: "test-pb-config", BuilderTTL: 1000, SchedulerTTL: 1000}
	err = mongoDBConnector.SaveConfig(conf)
	require.NoError(t, err)
	err = mongoDBConnector.UpdateBuilderTTL(conf.Name, 500)
	require.NoError(t, err)
	err = mongoDBConnector.UpdateSchedulerTTL(conf.Name, 200)
	require.NoError(t, err)
	conf, err = mongoDBConnector.GetConfig(conf.Name, pb.IdType_NAME)
	require.NoError(t, err)
	require.EqualValues(t, 500, conf.BuilderTTL)
	require.EqualValues(t, 200, conf.SchedulerTTL)
	err = mongoDBConnector.DeleteConfig(conf.Name, pb.IdType_NAME)
	require.NoError(t, err)
}
