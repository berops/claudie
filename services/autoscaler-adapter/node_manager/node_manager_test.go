package node_manager

import (
	"testing"

	"github.com/berops/claudie/proto/pb"
)

var (
	np = &pb.NodePool{
		Provider: &pb.Provider{
			SpecName:          "test",
			CloudProviderName: "aws",
			AwsAccessKey:      "",
			Credentials:       "",
		},
		ServerType: "t3.small",
		DiskSize:   50,
		Region:     "eu-north-1",
		AutoscalerConfig: &pb.AutoscalerConf{
			Min: 1,
			Max: 5,
		},
	}
)

func TestLabels(t *testing.T) {
	nm := NewNodeManager([]*pb.NodePool{np})
	t.Log(nm.GetCapacity(np))
}
