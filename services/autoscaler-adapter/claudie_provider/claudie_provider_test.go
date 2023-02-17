package claudie_provider

import (
	"testing"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/autoscaler-adapter/node_manager"
)

var np = &pb.NodePool{
	Name:       "test",
	Region:     "hel1",
	Zone:       "hel1-dc2",
	DiskSize:   50,
	Image:      "ubuntu",
	ServerType: "cpx11",
	AutoscalerConfig: &pb.AutoscalerConf{
		Min: 1,
		Max: 5,
	},
	Provider: &pb.Provider{
		CloudProviderName: "hetzner",
		SpecName:          "hetzner-test",
		Credentials:       "",
	},
}

func TestNodeManager(t *testing.T) {
	nm := node_manager.NewNodeManager([]*pb.NodePool{np})
	t.Log(nm.GetLabels(np))
	t.Log(nm.GetCapacity(np))
}
