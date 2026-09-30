package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateControlPlane verifies denied providers are rejected as control-plane nodepools.
func TestValidateControlPlane(t *testing.T) {
	for provider, wantErr := range map[string]bool{"vastai-1": true, "hetzner-1": false} {
		t.Run(provider, func(t *testing.T) {
			m := &Manifest{
				Providers: Provider{
					Hetzner: []Hetzner{{Name: "hetzner-1"}},
					VastAi:  []VastAi{{Name: "vastai-1"}},
				},
				NodePools: NodePool{Dynamic: []DynamicNodePool{{Name: "np", ProviderSpec: ProviderSpec{Name: provider}}}},
			}
			err := validateNodepools(m, &Cluster{Name: "cluster-1", Pools: Pool{Control: []string{"np"}}})
			if wantErr {
				require.ErrorContains(t, err, "control-plane")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
