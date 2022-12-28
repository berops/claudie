package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	testManifest                = &Manifest{NodePools: NodePool{Dynamic: []DynamicNodePool{{Name: "np1"}}}}
	testClusterVersionPass      = &Kubernetes{Clusters: []Cluster{{Name: "cluster1", Network: "10.0.0.0/8", Version: "v1.22.0", Pools: Pool{Control: []string{"np1"}}}}}
	testClusterVersionFailMinor = &Kubernetes{Clusters: []Cluster{{Name: "cluster1", Network: "10.0.0.0/8", Version: "v1.21.0", Pools: Pool{Control: []string{"np1"}}}}}
	testClusterVersionFailMajor = &Kubernetes{Clusters: []Cluster{{Name: "cluster1", Network: "10.0.0.0/8", Version: "v2.22.0", Pools: Pool{Control: []string{"np1"}}}}}
)

func TestKubernetes(t *testing.T) {
	err := testClusterVersionPass.Validate(testManifest)
	require.NoError(t, err)
	err = testClusterVersionFailMajor.Validate(testManifest)
	require.Error(t, err)
	err = testClusterVersionFailMinor.Validate(testManifest)
	require.Error(t, err)
}
