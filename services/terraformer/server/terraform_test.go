package main

import (
	"testing"

	"github.com/Berops/platform/proto/pb"
	"github.com/stretchr/testify/require"
)

var desiredState *pb.Project = &pb.Project{
	Name: "TestProjectName",
	Clusters: []*pb.Cluster{
		{
			Name:       "cluster1",
			Kubernetes: "20.1",
			Network:    "192.168.2.0/24",
			Kubeconfig: "ExampleKubeConfig",
			PublicKey:  "/Users/samuelstolicny/Github/Berops/platform/keys/testkey.pub",
			PrivateKey: "/Users/samuelstolicny/Github/Berops/platform/keys/testkey",
			NodePools: []*pb.NodePool{
				{
					Name:   "NodePoolName1",
					Region: "Autralia",
					Master: &pb.Node{
						Count:      1,
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Location:   "example loca",
						Datacenter: "example datacenter",
					},
					Worker: &pb.Node{
						Count:      2,
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Location:   "example loca",
						Datacenter: "example datacenter",
					},
					Provider: &pb.Provider{
						Name:        "hetzner",
						Credentials: "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1",
					},
				},
				{
					Name:   "NodePoolName2",
					Region: "Autralia",
					Master: &pb.Node{
						Count:      1,
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
					},
					Worker: &pb.Node{
						Count:      2,
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
					},
					Provider: &pb.Provider{
						Name:        "gcp",
						Credentials: "/Users/samuelstolicny/Github/Berops/platform/keys/platform-296509-d6ddeb344e91.json",
					},
				},
			},
		},
		{
			Name:       "cluster2",
			Kubernetes: "20.1",
			Network:    "192.168.2.0/24",
			Kubeconfig: "ExampleKubeConfig",
			PublicKey:  "/Users/samuelstolicny/Github/Berops/platform/keys/testkey.pub",
			PrivateKey: "/Users/samuelstolicny/Github/Berops/platform/keys/testkey",
			NodePools: []*pb.NodePool{
				{
					Name:   "NodePoolName3",
					Region: "Autralia",
					Master: &pb.Node{
						Count:      1,
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Location:   "example loca",
						Datacenter: "example datacenter",
					},
					Worker: &pb.Node{
						Count:      2,
						ServerType: "cpx11",
						Image:      "ubuntu-20.04",
						DiskSize:   20,
						Zone:       "example zone",
						Location:   "example loca",
						Datacenter: "example datacenter",
					},
					Provider: &pb.Provider{
						Name:        "hetzner",
						Credentials: "xIAfsb7M5K6etYAfXYcg5iYyrFGNlCxcICo060HVEygjoF0usFpv5P9X7pk85Xe1",
					},
				},
				{
					Name:   "NodePoolName4",
					Region: "Autralia",
					Master: &pb.Node{
						Count:      1,
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
					},
					Worker: &pb.Node{
						Count:      2,
						ServerType: "e2-small",
						Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
						DiskSize:   20,
					},
					Provider: &pb.Provider{
						Name:        "gcp",
						Credentials: "/Users/samuelstolicny/Github/Berops/platform/keys/platform-296509-d6ddeb344e91.json",
					},
				},
			},
		},
	},
}

func TestBuildInfrastructure(t *testing.T) {
	err := buildInfrastructure(desiredState)
	require.NoError(t, err)
}

func TestDestroyInfrastructure(t *testing.T) {
	err := destroyInfrastructure(desiredState)
	require.NoError(t, err)
}
