package claudieDB

import (
	"fmt"
	"testing"

	"github.com/Berops/platform/envs"
	"github.com/Berops/platform/proto/pb"
	"github.com/stretchr/testify/require"
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
								Name: "hetzner",
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
								Name: "hetzner",
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
								Name: "gcp",
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
								Name: "gcp",
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
								Name: "hetzner",
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
								Name: "hetzner",
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
								Name: "gcp",
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
								Name: "gcp",
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
								Name: "gcp",
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
						Name: "gcp",
					},
				},
			},
		},
	}
)

func TestSaveConfig(t *testing.T) {
	cm := ClaudieMongo{Url: envs.DatabaseURL}
	err := cm.Connect()
	require.NoError(t, err)
	err = cm.Init()
	require.NoError(t, err)
	defer cm.Disconnect()
	conf := &pb.Config{DesiredState: desiredState, Name: "test-pb-config"}
	cm.SaveConfig(conf)
	fmt.Println("Config id: " + conf.Id)
	require.NotEmpty(t, conf.Id)
	err = cm.DeleteConfig(conf.Name, pb.IdType_NAME)
	require.NoError(t, err)
}
