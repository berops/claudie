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

	verdaManifest := &Manifest{
		Providers: Provider{
			Verda: []Verda{{
				Name:         "verda-1",
				ClientId:     "fake-client-id",
				ClientSecret: "fake-client-secret",
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

	awsManifest := &Manifest{
		Providers: Provider{
			AWS: []AWS{{
				Name:      "aws-1",
				AccessKey: "fake-access-key-1234",
				SecretKey: "fake-secret-key-fake-secret-key-fake-secr",
			}},
		},
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{
					Name:    "cluster-1",
					Network: "10.0.0.0/8",
					Version: "v1.33.0",
					Pools:   Pool{Compute: []string{"worker-np"}},
				},
			},
		},
	}

	azureManifest := &Manifest{
		Providers: Provider{
			Azure: []Azure{{
				Name:           "azure-1",
				SubscriptionId: "fake-subscription-id",
				TenantId:       "fake-tenant-id",
				ClientId:       "fake-client-id",
				ClientSecret:   "fake-client-secret",
			}},
		},
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{
					Name:    "cluster-1",
					Network: "10.0.0.0/8",
					Version: "v1.33.0",
					Pools:   Pool{Compute: []string{"worker-np"}},
				},
			},
		},
	}

	ociManifest := &Manifest{
		Providers: Provider{
			OCI: []OCI{{
				Name:           "oci-1",
				PrivateKey:     "fake-private-key",
				KeyFingerprint: "fake-fingerprint",
				TenancyOCID:    "fake-tenancy-ocid",
				UserOCID:       "fake-user-ocid",
				CompartmentID:  "fake-compartment-ocid",
			}},
		},
		Kubernetes: Kubernetes{
			Clusters: []Cluster{
				{
					Name:    "cluster-1",
					Network: "10.0.0.0/8",
					Version: "v1.33.0",
					Pools:   Pool{Compute: []string{"worker-np"}},
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
			name: "spot on Verda worker pool passes",
			nodepool: &DynamicNodePool{
				Name:       "worker-np",
				ServerType: "Standard-1",
				Image:      "ubuntu-22.04",
				Count:      1,
				Spot:       true,
				ProviderSpec: ProviderSpec{
					Name:   "verda-1",
					Region: "eu-north-1",
				},
			},
			manifest:  verdaManifest,
			wantError: false,
		},
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
			name: "spot on AWS worker pool passes",
			nodepool: &DynamicNodePool{
				Name:       "worker-np",
				ServerType: "t3.medium",
				Image:      "ami-fake",
				Count:      1,
				Spot:       true,
				ProviderSpec: ProviderSpec{
					Name:   "aws-1",
					Region: "eu-central-1",
					Zone:   "eu-central-1a",
				},
			},
			manifest:  awsManifest,
			wantError: false,
		},
		{
			name: "spot on Azure worker pool passes",
			nodepool: &DynamicNodePool{
				Name:       "worker-np",
				ServerType: "Standard_B2s",
				Image:      "Canonical:0001-com-ubuntu-server-jammy:22_04-lts:latest",
				Count:      1,
				Spot:       true,
				ProviderSpec: ProviderSpec{
					Name:   "azure-1",
					Region: "germanywestcentral",
				},
			},
			manifest:  azureManifest,
			wantError: false,
		},
		{
			name: "spot on OCI worker pool passes",
			nodepool: &DynamicNodePool{
				Name:       "worker-np",
				ServerType: "VM.Standard.E4.Flex",
				Image:      "ocid1.image.fake",
				Count:      1,
				Spot:       true,
				ProviderSpec: ProviderSpec{
					Name:   "oci-1",
					Region: "eu-frankfurt-1",
				},
			},
			manifest:  ociManifest,
			wantError: false,
		},
		{
			name: "spot on unsupported provider fails",
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
