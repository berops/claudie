package serializer_test

import (
	"testing"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/serializer"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestFileSerializer(t *testing.T) {
	t.Parallel()

	binaryFile := "../tmp/project.bin"
	jsonFile := "../tmp/project.json"

	providers := make(map[string]*pb.Provider) //create a new map of providers
	providers["hetzner"] = &pb.Provider{       //add new provider to the map
		Name: "hetzner",
		ControlNodeSpecs: &pb.ControlNodeSpecs{
			Count:      1,
			ServerType: "cpx11",
			Image:      "ubuntu-20.04",
		},
		ComputeNodeSpecs: &pb.ComputeNodeSpecs{
			Count:      2,
			ServerType: "cpx11",
			Image:      "ubuntu-20.04",
		},
		IsInUse: true,
	}
	providers["gcp"] = &pb.Provider{
		Name: "gcp",
		ControlNodeSpecs: &pb.ControlNodeSpecs{
			Count:      0,
			ServerType: "e2-small",
			Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
		},
		ComputeNodeSpecs: &pb.ComputeNodeSpecs{
			Count:      0,
			ServerType: "f1-micro",
			Image:      "ubuntu-os-cloud/ubuntu-2004-lts",
		},
		IsInUse: false,
	}

	project := &pb.Project{ //create a new project
		Metadata: &pb.Metadata{
			Name: "ProjectX",
			Id:   "12345",
		},
		Cluster: &pb.Cluster{
			Network: &pb.Network{
				Ip:   "192.168.2.0",
				Mask: "24",
			},
			Nodes: []*pb.Node{
				{
					PrivateIp: "192.168.2.1",
				},
				{
					PrivateIp: "192.168.2.2",
				},
				{
					PrivateIp: "192.168.2.3",
				},
			},
			KubernetesVersion: "v1.19.0",
			Providers:         providers,
			PrivateKey:        "/Users/samuelstolicny/go/src/github.com/Berops/platform/keys/testkey",
			PublicKey:         "/Users/samuelstolicny/go/src/github.com/Berops/platform/keys/testkey.pub",
		},
	}

	err := serializer.WriteProtobufToBinaryFile(project, binaryFile)
	require.NoError(t, err)

	err = serializer.WriteProtobufToJSONFile(project, jsonFile)
	require.NoError(t, err)

	readProject := &pb.Project{}
	err = serializer.ReadProtobufFromBinaryFile(readProject, binaryFile)
	require.NoError(t, err)

	require.True(t, proto.Equal(project, readProject))
}
