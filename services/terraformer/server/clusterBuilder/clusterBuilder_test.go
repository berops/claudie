package clusterBuilder

import (
	"fmt"
	"testing"

	"github.com/Berops/claudie/internal/templateUtils"
	"github.com/Berops/claudie/proto/pb"
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

func TestFillNodes(t *testing.T) {
	out, err := readIPs(jsonData)
	if err == nil {
		var m = &pb.NodePool{}
		fillNodes(&out, m)
		fmt.Println(m)
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
