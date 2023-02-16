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

	testNodepoolAutoScalerSuccAC = &DynamicNodePool{Name: "Test", ServerType: "s1", Image: "ubuntu", DiskSize: 50, AutoscalerConfig: AutoscalerConfig{Min: 1, Max: 3}, ProviderSpec: ProviderSpec{Name: "p1", Region: "a", Zone: "1"}}
	testNodepoolAutoScalerSuccC  = &DynamicNodePool{Name: "Test", ServerType: "s1", Image: "ubuntu", DiskSize: 50, Count: 1, ProviderSpec: ProviderSpec{Name: "p1", Region: "a", Zone: "1"}}
	testNodepoolAutoScalerFail   = &DynamicNodePool{Name: "Test", ServerType: "s1", Image: "ubuntu", DiskSize: 50, Count: 1, AutoscalerConfig: AutoscalerConfig{Min: 1, Max: 3}, ProviderSpec: ProviderSpec{Name: "p1", Region: "a", Zone: "1"}}
	testDomainFail               = &Manifest{
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{Name: "VERY-LONG-NAME-FOR-CLUSTER", Pools: Pool{
					Control: []string{"VERY-LONG-NAME-FOR-NODE-POOL1-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
					Compute: []string{"VERY-LONG-NAME-FOR-NODE-POOL2-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
				}},
			},
		},
		NodePools: NodePool{
			Dynamic: []DynamicNodePool{
				{Name: "VERY-LONG-NAME-FOR-NODE-POOL1-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Count: 10},
				{Name: "VERY-LONG-NAME-FOR-NODE-POOL2-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Count: 100},
			},
		},
	}

	testDomainSuccess = &Manifest{
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{Name: "CLUSTER", Pools: Pool{
					Control: []string{"POOL-1"},
					Compute: []string{"POOL-2"},
				}},
			},
		},
		NodePools: NodePool{
			Dynamic: []DynamicNodePool{
				{Name: "POOL-1", Count: 10},
				{Name: "POOL-2", Count: 100},
			},
		},
	}
)

// TestDomain tests the domain which will be formed from node name
func TestDomain(t *testing.T) {
	err := checkLengthOfFutureDomain(testDomainSuccess)
	require.NoError(t, err)
	err = checkLengthOfFutureDomain(testDomainFail)
	require.Error(t, err)
}

// TestKubernetes tests the kubernetes version validation
func TestKubernetes(t *testing.T) {
	err := testClusterVersionPass.Validate(testManifest)
	require.NoError(t, err)
	err = testClusterVersionFailMajor.Validate(testManifest)
	require.Error(t, err)
	err = testClusterVersionFailMinor.Validate(testManifest)
	require.Error(t, err)
}

// TestNodepool tests the nodepool spec validation
func TestNodepool(t *testing.T) {
	err := testNodepoolAutoScalerSuccAC.Validate()
	require.NoError(t, err)
	err = testNodepoolAutoScalerSuccC.Validate()
	require.NoError(t, err)
	err = testNodepoolAutoScalerFail.Validate()
	require.Error(t, err)
}
