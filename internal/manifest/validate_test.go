package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	testDomainFail = &Manifest{
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{Name: "VERY-LONG-NAME-FOR-CLUSTER", Pools: Pool{
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

func TestDomain(t *testing.T) {
	err := checkLengthOfFutureDomain(testDomainSuccess)
	require.NoError(t, err)
	err = checkLengthOfFutureDomain(testDomainFail)
	require.Error(t, err)
}
