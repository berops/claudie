package clusterBuilder

import (
	"fmt"
	"testing"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb"
	"github.com/stretchr/testify/require"
)

var jsonData = "{\"compute\":{\"test-cluster-compute1\":\"0.0.0.65\",\n\"test-cluster-compute2\":\"0.0.0.512\"},\n\"control\":{\"test-cluster-control1\":\"0.0.0.72\",\n\"test-cluster-control2\":\"0.0.0.65\"}}"

var testNp = &pb.NodePool{
	Name:       "test-np",
	Region:     "West Europe",
	ServerType: "Standard_E64s_v3",
	Image:      "Canonical:0001-com-ubuntu-minimal-focal:minimal-20_04-lts:20.04.202004230",
	DiskSize:   50,
	Zone:       "1",
	Count:      3,
	Nodes:      []*pb.Node{},
	IsControl:  true,
	Provider: &pb.Provider{
		CloudProviderName: "azure",
		Credentials:       "",
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
	templateLoader := templateUtils.TemplateLoader{Directory: "../../templates"}
	template := templateUtils.Templates{Directory: "."}
	tpl, err := templateLoader.LoadTemplate("azure.tpl")
	require.NoError(t, err)
	err = template.Generate(tpl, "az-acc-net.tf", &NodepoolsData{ClusterName: "test", ClusterHash: "abcdef", NodePools: []*pb.NodePool{testNp}})
	require.NoError(t, err)
}

// TestGetCIDR tests getCIDR function
func TestGetCIDR(t *testing.T) {
	type testCase struct {
		desc     string
		baseCIDR string
		position int
		value    int
		out      string
	}

	testDataSucc := []testCase{
		{
			desc:     "Second octet change",
			baseCIDR: "10.0.0.0/24",
			position: 1,
			value:    10,
			out:      "10.10.0.0/24",
		},

		{
			desc:     "Third octet change",
			baseCIDR: "10.0.0.0/24",
			position: 2,
			value:    10,
			out:      "10.0.10.0/24",
		},
	}
	for _, test := range testDataSucc {
		if out, err := getCIDR(test.baseCIDR, test.position, test.value); out != test.out || err != nil {
			t.Error(test.desc, err)
		}
	}
	testDataFail := []testCase{
		{
			desc:     "Third octet invalid change",
			baseCIDR: "10.0.0.0/24",
			position: 2,
			value:    300,
			out:      "10.0.300.0/24",
		},
		{
			desc:     "Invalid base CIDR",
			baseCIDR: "300.0.0.0/24",
			position: 2,
			value:    10,
			out:      "10.0.10.0/24",
		},
	}
	for _, test := range testDataFail {
		if _, err := getCIDR(test.baseCIDR, test.position, test.value); err == nil {
			t.Error(test.desc, fmt.Errorf("test should have failed, but was successful"))
		}
	}
}
