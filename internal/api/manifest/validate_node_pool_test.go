package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidateSpot verifies the spot instance constraints for dynamic nodepools.
func TestValidateSpot(t *testing.T) {
	gcpManifest := &Manifest{
		Providers: Provider{
			GCP: []GCP{{
				Name:        "gcp-1",
				Credentials: "fake-credentials",
				GCPProject:  "fake-project",
			}},
		},
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{
					Name:    "cluster-1",
					Network: "10.0.0.0/8",
					Version: "v1.33.0",
					Pools: Pool{
						Control: []string{"control-np"},
						Compute: []string{"worker-np"},
					},
				},
			},
		},
	}

	hetznerManifest := &Manifest{
		Providers: Provider{
			Hetzner: []Hetzner{{
				Name:        "hetzner-1",
				Credentials: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			}},
		},
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{
					Name:    "cluster-1",
					Network: "10.0.0.0/8",
					Version: "v1.33.0",
					Pools: Pool{
						Compute: []string{"worker-np"},
					},
				},
			},
		},
	}

	cases := []struct {
		name            string
		nodepool        *DynamicNodePool
		manifest        *Manifest
		wantError       bool
		wantErrContains string
	}{
		{
			name: "spot on GCP worker pool passes",
			nodepool: &DynamicNodePool{
				Name:       "worker-np",
				ServerType: "e2-medium",
				Image:      "ubuntu-2204",
				Count:      1,
				Spot:       true,
				ProviderSpec: ProviderSpec{
					Name:   "gcp-1",
					Region: "us-central1",
					Zone:   "us-central1-a",
				},
			},
			manifest:  gcpManifest,
			wantError: false,
		},
		{
			name: "spot on non-GCP provider fails",
			nodepool: &DynamicNodePool{
				Name:       "worker-np",
				ServerType: "cx21",
				Image:      "ubuntu-22.04",
				Count:      1,
				Spot:       true,
				ProviderSpec: ProviderSpec{
					Name:   "hetzner-1",
					Region: "fsn1",
					Zone:   "fsn1-dc14",
				},
			},
			manifest:        hetznerManifest,
			wantError:       true,
			wantErrContains: "only supported on GCP",
		},
		{
			name: "spot on GCP control-plane pool fails",
			nodepool: &DynamicNodePool{
				Name:       "control-np",
				ServerType: "e2-medium",
				Image:      "ubuntu-2204",
				Count:      1,
				Spot:       true,
				ProviderSpec: ProviderSpec{
					Name:   "gcp-1",
					Region: "us-central1",
					Zone:   "us-central1-a",
				},
			},
			manifest:        gcpManifest,
			wantError:       true,
			wantErrContains: "not allowed on control-plane",
		},
		{
			name: "spot=false on any provider passes",
			nodepool: &DynamicNodePool{
				Name:       "worker-np",
				ServerType: "cx21",
				Image:      "ubuntu-22.04",
				Count:      1,
				Spot:       false,
				ProviderSpec: ProviderSpec{
					Name:   "hetzner-1",
					Region: "fsn1",
					Zone:   "fsn1-dc14",
				},
			},
			manifest:  hetznerManifest,
			wantError: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Call Validate (not validateSpot directly) so the test also
			// catches regressions if Validate stops invoking validateSpot.
			err := tc.nodepool.Validate(tc.manifest)
			if tc.wantError {
				require.Error(t, err, "expected error but got nil")
				if tc.wantErrContains != "" {
					require.ErrorContains(t, err, tc.wantErrContains)
				}
			} else {
				require.NoError(t, err, "expected no error but got: %v", err)
			}
		})
	}
}
