package cluster_builder

import (
	"fmt"
	"testing"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/templates"
	"github.com/stretchr/testify/require"
)

var jsonData = "{\"compute\":{\"test-cluster-compute1\":\"0.0.0.65\",\n\"test-cluster-compute2\":\"0.0.0.512\"},\n\"control\":{\"test-cluster-control1\":\"0.0.0.72\",\n\"test-cluster-control2\":\"0.0.0.65\"}}"

var testNp = &pb.NodePool{
	Name:      "test-np",
	Nodes:     []*pb.Node{},
	IsControl: true,
	NodePoolType: &pb.NodePool_DynamicNodePool{
		DynamicNodePool: &pb.DynamicNodePool{
			Region:          "West Europe",
			ServerType:      "Standard_E64s_v3",
			Image:           "Canonical:0001-com-ubuntu-minimal-focal:minimal-20_04-lts:20.04.202004230",
			StorageDiskSize: 50,
			Zone:            "1",
			Count:           3,
			Provider: &pb.Provider{
				CloudProviderName: "azure",
				Credentials:       "",
			},
		},
	},
}

func TestReadOutput(t *testing.T) {
	out, err := readIPs(jsonData)
	if err == nil {
		t.Log(out.IPs)
	}
	require.NoError(t, err)
}

func TestGenerateTf(t *testing.T) {
	template := templateUtils.Templates{Directory: "."}
	file, err := templates.CloudProviderTemplates.ReadFile("azure/k8s.tpl")
	require.NoError(t, err)
	tpl, err := templateUtils.LoadTemplate(string(file))
	require.NoError(t, err)
	err = template.Generate(tpl, "az-acc-net.tf", &NodepoolsData{ClusterName: "test", ClusterHash: "abcdef", NodePools: []NodePoolInfo{{NodePool: testNp.GetDynamicNodePool()}}})
	require.NoError(t, err)
}

// TestGetCIDR tests getCIDR function
func TestGetCIDR(t *testing.T) {
	type testCase struct {
		desc     string
		baseCIDR string
		position int
		existing map[string]struct{}
		out      string
	}

	testDataSucc := []testCase{
		{
			desc:     "Second octet change",
			baseCIDR: "10.0.0.0/24",
			position: 1,
			existing: map[string]struct{}{
				"10.1.0.0/24": {},
			},
			out: "10.0.0.0/24",
		},

		{
			desc:     "Third octet change",
			baseCIDR: "10.0.0.0/24",
			position: 2,
			existing: map[string]struct{}{
				"10.0.0.0/24": {},
			},
			out: "10.0.1.0/24",
		},
	}
	for _, test := range testDataSucc {
		if out, err := getCIDR(test.baseCIDR, test.position, test.existing); out != test.out || err != nil {
			t.Error(test.desc, err, out)
		}
	}
	testDataFail := []testCase{
		{
			desc:     "Max IP error",
			baseCIDR: "10.0.0.0/24",
			position: 2,
			existing: func() map[string]struct{} {
				m := make(map[string]struct{})
				for i := 0; i < 256; i++ {
					m[fmt.Sprintf("10.0.%d.0/24", i)] = struct{}{}
				}
				return m
			}(),
			out: "",
		},
		{
			desc:     "Invalid base CIDR",
			baseCIDR: "300.0.0.0/24",
			position: 2,
			existing: map[string]struct{}{
				"10.0.0.0/24": {},
			},
			out: "10.0.10.0/24",
		},
	}
	for _, test := range testDataFail {
		if _, err := getCIDR(test.baseCIDR, test.position, test.existing); err == nil {
			t.Error(test.desc, "test should have failed, but was successful")
		} else {
			t.Log(err)
		}
	}
}
