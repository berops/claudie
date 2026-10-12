package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateLoadBalancer verifies denied providers are rejected as load balancer nodepools.
func TestValidateLoadBalancer(t *testing.T) {
	for provider, wantErr := range map[string]bool{
		"vastai-1":    true,
		"ovh-1":       true,
		"verda-1":     true,
		"cloudrift-1": true,
		"gcp-1":       false,
	} {
		t.Run(provider, func(t *testing.T) {
			m := &Manifest{
				Providers: Provider{
					GCP:       []GCP{{Name: "gcp-1"}},
					VastAi:    []VastAi{{Name: "vastai-1"}},
					OVH:       []OVH{{Name: "ovh-1"}},
					Verda:     []Verda{{Name: "verda-1"}},
					CloudRift: []CloudRift{{Name: "cloudrift-1"}},
				},
				NodePools:  NodePool{Dynamic: []DynamicNodePool{{Name: "np", ProviderSpec: ProviderSpec{Name: provider}}}},
				Kubernetes: Kubernetes{Clusters: []Cluster{{Name: "cluster-1"}}},
			}
			lb := &LoadBalancer{Clusters: []LoadBalancerCluster{{
				Name:        "lb-1",
				DNS:         DNS{DNSZone: "example.com", Provider: "gcp-1"},
				TargetedK8s: "cluster-1",
				Pools:       []string{"np"},
			}}}
			err := lb.Validate(m)
			if wantErr {
				require.ErrorContains(t, err, "load balancer")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
