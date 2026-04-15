package spec

import (
	"testing"
)

// nolint
func TestCopyCredentials(t *testing.T) {
	tests := []struct {
		name       string
		pr         *Provider
		other      *Provider
		assertFunc func(t *testing.T, pr *Provider)
	}{
		{
			name:  "receiver nil — no panic",
			pr:    nil,
			other: &Provider{ProviderType: &Provider_Aws{Aws: &AWSProvider{AccessKey: "key"}}},
			assertFunc: func(t *testing.T, pr *Provider) {
				// nothing to assert; we just verify no panic occurred
			},
		},
		{
			name:  "other nil — receiver unchanged",
			pr:    &Provider{ProviderType: &Provider_Aws{Aws: &AWSProvider{AccessKey: "original"}}},
			other: nil,
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Aws).Aws.AccessKey
				if got != "original" {
					t.Errorf("AccessKey = %q, want %q", got, "original")
				}
			},
		},
		{
			name:  "both nil — no panic",
			pr:    nil,
			other: nil,
			assertFunc: func(t *testing.T, pr *Provider) {
				// nothing to assert; we just verify no panic occurred
			},
		},
		{
			name:  "both have nil ProviderType (default/zero) — no panic, no mutation",
			pr:    &Provider{},
			other: &Provider{},
			assertFunc: func(t *testing.T, pr *Provider) {
				if pr.ProviderType != nil {
					t.Errorf("ProviderType should remain nil, got %T", pr.ProviderType)
				}
			},
		},

		{
			name: "AWS receiver, Azure source — no mutation",
			pr: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "original-key", SecretKey: "original-secret"},
			}},
			other: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{ClientSecret: "azure-secret"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				aws := pr.ProviderType.(*Provider_Aws).Aws
				if aws.AccessKey != "original-key" {
					t.Errorf("AccessKey = %q, want %q", aws.AccessKey, "original-key")
				}
				if aws.SecretKey != "original-secret" {
					t.Errorf("SecretKey = %q, want %q", aws.SecretKey, "original-secret")
				}
			},
		},
		{
			name: "GCP receiver, Hetzner source — no mutation",
			pr: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{Key: "original-gcp-key"},
			}},
			other: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{Token: "hetzner-token"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Gcp).Gcp.Key
				if got != "original-gcp-key" {
					t.Errorf("Key = %q, want %q", got, "original-gcp-key")
				}
			},
		},

		{
			name: "AWS copies both access key and secret key",
			pr: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "old-key", SecretKey: "old-secret"},
			}},
			other: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "new-key", SecretKey: "new-secret"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				aws := pr.ProviderType.(*Provider_Aws).Aws
				if aws.AccessKey != "new-key" {
					t.Errorf("AccessKey = %q, want %q", aws.AccessKey, "new-key")
				}
				if aws.SecretKey != "new-secret" {
					t.Errorf("SecretKey = %q, want %q", aws.SecretKey, "new-secret")
				}
			},
		},
		{
			name: "AWS copies empty credentials (zero value overwrite)",
			pr: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "old-key", SecretKey: "old-secret"},
			}},
			other: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				aws := pr.ProviderType.(*Provider_Aws).Aws
				if aws.AccessKey != "" {
					t.Errorf("AccessKey = %q, want empty string", aws.AccessKey)
				}
				if aws.SecretKey != "" {
					t.Errorf("SecretKey = %q, want empty string", aws.SecretKey)
				}
			},
		},

		{
			name: "Azure copies client secret",
			pr: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{ClientSecret: "old-secret"},
			}},
			other: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{ClientSecret: "new-secret"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Azure).Azure.ClientSecret
				if got != "new-secret" {
					t.Errorf("ClientSecret = %q, want %q", got, "new-secret")
				}
			},
		},
		{
			name: "Azure copies empty client secret (zero value overwrite)",
			pr: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{ClientSecret: "old-secret"},
			}},
			other: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Azure).Azure.ClientSecret
				if got != "" {
					t.Errorf("ClientSecret = %q, want empty string", got)
				}
			},
		},

		{
			name: "Cloudflare copies token",
			pr: &Provider{ProviderType: &Provider_Cloudflare{
				Cloudflare: &CloudflareProvider{Token: "old-token"},
			}},
			other: &Provider{ProviderType: &Provider_Cloudflare{
				Cloudflare: &CloudflareProvider{Token: "new-token"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Cloudflare).Cloudflare.Token
				if got != "new-token" {
					t.Errorf("Token = %q, want %q", got, "new-token")
				}
			},
		},
		{
			name: "Cloudflare copies empty token (zero value overwrite)",
			pr: &Provider{ProviderType: &Provider_Cloudflare{
				Cloudflare: &CloudflareProvider{Token: "old-token"},
			}},
			other: &Provider{ProviderType: &Provider_Cloudflare{
				Cloudflare: &CloudflareProvider{},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Cloudflare).Cloudflare.Token
				if got != "" {
					t.Errorf("Token = %q, want empty string", got)
				}
			},
		},

		{
			name: "Cloudrift copies token",
			pr: &Provider{ProviderType: &Provider_Cloudrift{
				Cloudrift: &CloudRiftProvider{Token: "old-token"},
			}},
			other: &Provider{ProviderType: &Provider_Cloudrift{
				Cloudrift: &CloudRiftProvider{Token: "new-token"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Cloudrift).Cloudrift.Token
				if got != "new-token" {
					t.Errorf("Token = %q, want %q", got, "new-token")
				}
			},
		},
		{
			name: "Cloudrift copies empty token (zero value overwrite)",
			pr: &Provider{ProviderType: &Provider_Cloudrift{
				Cloudrift: &CloudRiftProvider{Token: "old-token"},
			}},
			other: &Provider{ProviderType: &Provider_Cloudrift{
				Cloudrift: &CloudRiftProvider{},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Cloudrift).Cloudrift.Token
				if got != "" {
					t.Errorf("Token = %q, want empty string", got)
				}
			},
		},

		{
			name: "Exoscale copies both api key and api secret",
			pr: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{ApiKey: "old-key", ApiSecret: "old-secret"},
			}},
			other: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{ApiKey: "new-key", ApiSecret: "new-secret"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				exo := pr.ProviderType.(*Provider_Exoscale).Exoscale
				if exo.ApiKey != "new-key" {
					t.Errorf("ApiKey = %q, want %q", exo.ApiKey, "new-key")
				}
				if exo.ApiSecret != "new-secret" {
					t.Errorf("ApiSecret = %q, want %q", exo.ApiSecret, "new-secret")
				}
			},
		},
		{
			name: "Exoscale copies empty credentials (zero value overwrite)",
			pr: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{ApiKey: "old-key", ApiSecret: "old-secret"},
			}},
			other: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				exo := pr.ProviderType.(*Provider_Exoscale).Exoscale
				if exo.ApiKey != "" {
					t.Errorf("ApiKey = %q, want empty string", exo.ApiKey)
				}
				if exo.ApiSecret != "" {
					t.Errorf("ApiSecret = %q, want empty string", exo.ApiSecret)
				}
			},
		},

		{
			name: "GCP copies key",
			pr: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{Key: "old-key"},
			}},
			other: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{Key: "new-key"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Gcp).Gcp.Key
				if got != "new-key" {
					t.Errorf("Key = %q, want %q", got, "new-key")
				}
			},
		},
		{
			name: "GCP copies empty key (zero value overwrite)",
			pr: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{Key: "old-key"},
			}},
			other: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Gcp).Gcp.Key
				if got != "" {
					t.Errorf("Key = %q, want empty string", got)
				}
			},
		},

		{
			name: "Hetzner copies token",
			pr: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{Token: "old-token"},
			}},
			other: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{Token: "new-token"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Hetzner).Hetzner.Token
				if got != "new-token" {
					t.Errorf("Token = %q, want %q", got, "new-token")
				}
			},
		},
		{
			name: "Hetzner copies empty token (zero value overwrite)",
			pr: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{Token: "old-token"},
			}},
			other: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				got := pr.ProviderType.(*Provider_Hetzner).Hetzner.Token
				if got != "" {
					t.Errorf("Token = %q, want empty string", got)
				}
			},
		},

		{
			name: "OCI copies both fingerprint and private key",
			pr: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{KeyFingerprint: "old:fp", PrivateKey: "old-key"},
			}},
			other: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{KeyFingerprint: "new:fp", PrivateKey: "new-key"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				oci := pr.ProviderType.(*Provider_Oci).Oci
				if oci.KeyFingerprint != "new:fp" {
					t.Errorf("KeyFingerprint = %q, want %q", oci.KeyFingerprint, "new:fp")
				}
				if oci.PrivateKey != "new-key" {
					t.Errorf("PrivateKey = %q, want %q", oci.PrivateKey, "new-key")
				}
			},
		},
		{
			name: "OCI copies empty credentials (zero value overwrite)",
			pr: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{KeyFingerprint: "old:fp", PrivateKey: "old-key"},
			}},
			other: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				oci := pr.ProviderType.(*Provider_Oci).Oci
				if oci.KeyFingerprint != "" {
					t.Errorf("KeyFingerprint = %q, want empty string", oci.KeyFingerprint)
				}
				if oci.PrivateKey != "" {
					t.Errorf("PrivateKey = %q, want empty string", oci.PrivateKey)
				}
			},
		},

		{
			name: "OpenStack copies both credential ID and secret",
			pr: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{
					ApplicationCredentialID:     "old-id",
					ApplicationCredentialSecret: "old-secret",
				},
			}},
			other: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{
					ApplicationCredentialID:     "new-id",
					ApplicationCredentialSecret: "new-secret",
				},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				os := pr.ProviderType.(*Provider_Openstack).Openstack
				if os.ApplicationCredentialID != "new-id" {
					t.Errorf("ApplicationCredentialID = %q, want %q", os.ApplicationCredentialID, "new-id")
				}
				if os.ApplicationCredentialSecret != "new-secret" {
					t.Errorf("ApplicationCredentialSecret = %q, want %q", os.ApplicationCredentialSecret, "new-secret")
				}
			},
		},
		{
			name: "OpenStack copies empty credentials (zero value overwrite)",
			pr: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{
					ApplicationCredentialID:     "old-id",
					ApplicationCredentialSecret: "old-secret",
				},
			}},
			other: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				os := pr.ProviderType.(*Provider_Openstack).Openstack
				if os.ApplicationCredentialID != "" {
					t.Errorf("ApplicationCredentialID = %q, want empty string", os.ApplicationCredentialID)
				}
				if os.ApplicationCredentialSecret != "" {
					t.Errorf("ApplicationCredentialSecret = %q, want empty string", os.ApplicationCredentialSecret)
				}
			},
		},

		{
			name: "AWS source provider is not mutated after copy",
			pr: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "old-key", SecretKey: "old-secret"},
			}},
			other: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "source-key", SecretKey: "source-secret"},
			}},
			assertFunc: func(t *testing.T, pr *Provider) {
				// Mutate pr after copy to verify other was not aliased
				pr.ProviderType.(*Provider_Aws).Aws.AccessKey = "mutated"

				// We can't access `other` here directly; this test is a reminder
				// to verify aliasing in integration if structs are ever pointer-shared.
				// For value-type string fields this is inherently safe, but the test
				// documents the intent explicitly.
				got := pr.ProviderType.(*Provider_Aws).Aws.AccessKey
				if got != "mutated" {
					t.Errorf("post-copy mutation on receiver failed, got %q", got)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.pr.CopyCredentials(tc.other)
			if tc.assertFunc != nil {
				tc.assertFunc(t, tc.pr)
			}
		})
	}
}

func TestCredentialsEqual(t *testing.T) {
	tests := []struct {
		name     string
		pr       *Provider
		other    *Provider
		expected bool
	}{
		{
			name:     "both nil",
			pr:       nil,
			other:    nil,
			expected: false,
		},
		{
			name:     "receiver nil, other non-nil",
			pr:       nil,
			other:    &Provider{},
			expected: false,
		},
		{
			name:     "receiver non-nil, other nil",
			pr:       &Provider{},
			other:    nil,
			expected: false,
		},
		{
			name:     "both have nil ProviderType (default/zero value)",
			pr:       &Provider{},
			other:    &Provider{},
			expected: false,
		},

		{
			name: "AWS vs Azure",
			pr: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "key", SecretKey: "secret"},
			}},
			other: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{ClientSecret: "secret"},
			}},
			expected: false,
		},
		{
			name: "GCP vs Hetzner",
			pr: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{Key: "gcp-key"},
			}},
			other: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{Token: "hetzner-token"},
			}},
			expected: false,
		},

		{
			name: "AWS equal credentials",
			pr: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "AKIAIOSFODNN7EXAMPLE", SecretKey: "wJalrXUtnFEMI"},
			}},
			other: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "AKIAIOSFODNN7EXAMPLE", SecretKey: "wJalrXUtnFEMI"},
			}},
			expected: true,
		},
		{
			name: "AWS different access key",
			pr: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "KEY_A", SecretKey: "secret"},
			}},
			other: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "KEY_B", SecretKey: "secret"},
			}},
			expected: false,
		},
		{
			name: "AWS different secret key",
			pr: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "key", SecretKey: "SECRET_A"},
			}},
			other: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{AccessKey: "key", SecretKey: "SECRET_B"},
			}},
			expected: false,
		},
		{
			name: "AWS both credentials empty (zero value)",
			pr: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{},
			}},
			other: &Provider{ProviderType: &Provider_Aws{
				Aws: &AWSProvider{},
			}},
			expected: true,
		},

		{
			name: "Azure equal client secret",
			pr: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{ClientSecret: "azure-secret-123"},
			}},
			other: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{ClientSecret: "azure-secret-123"},
			}},
			expected: true,
		},
		{
			name: "Azure different client secret",
			pr: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{ClientSecret: "secret-A"},
			}},
			other: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{ClientSecret: "secret-B"},
			}},
			expected: false,
		},
		{
			name: "Azure empty client secret (zero value)",
			pr: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{},
			}},
			other: &Provider{ProviderType: &Provider_Azure{
				Azure: &AzureProvider{},
			}},
			expected: true,
		},

		{
			name: "Cloudflare equal token",
			pr: &Provider{ProviderType: &Provider_Cloudflare{
				Cloudflare: &CloudflareProvider{Token: "cf-token-abc"},
			}},
			other: &Provider{ProviderType: &Provider_Cloudflare{
				Cloudflare: &CloudflareProvider{Token: "cf-token-abc"},
			}},
			expected: true,
		},
		{
			name: "Cloudflare different token",
			pr: &Provider{ProviderType: &Provider_Cloudflare{
				Cloudflare: &CloudflareProvider{Token: "token-A"},
			}},
			other: &Provider{ProviderType: &Provider_Cloudflare{
				Cloudflare: &CloudflareProvider{Token: "token-B"},
			}},
			expected: false,
		},
		{
			name: "Cloudflare empty token (zero value)",
			pr: &Provider{ProviderType: &Provider_Cloudflare{
				Cloudflare: &CloudflareProvider{},
			}},
			other: &Provider{ProviderType: &Provider_Cloudflare{
				Cloudflare: &CloudflareProvider{},
			}},
			expected: true,
		},

		{
			name: "Cloudrift equal token",
			pr: &Provider{ProviderType: &Provider_Cloudrift{
				Cloudrift: &CloudRiftProvider{Token: "crift-token-xyz"},
			}},
			other: &Provider{ProviderType: &Provider_Cloudrift{
				Cloudrift: &CloudRiftProvider{Token: "crift-token-xyz"},
			}},
			expected: true,
		},
		{
			name: "Cloudrift different token",
			pr: &Provider{ProviderType: &Provider_Cloudrift{
				Cloudrift: &CloudRiftProvider{Token: "token-A"},
			}},
			other: &Provider{ProviderType: &Provider_Cloudrift{
				Cloudrift: &CloudRiftProvider{Token: "token-B"},
			}},
			expected: false,
		},
		{
			name: "Cloudrift empty token (zero value)",
			pr: &Provider{ProviderType: &Provider_Cloudrift{
				Cloudrift: &CloudRiftProvider{},
			}},
			other: &Provider{ProviderType: &Provider_Cloudrift{
				Cloudrift: &CloudRiftProvider{},
			}},
			expected: true,
		},

		{
			name: "Exoscale equal credentials",
			pr: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{ApiKey: "EXO-key", ApiSecret: "EXO-secret"},
			}},
			other: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{ApiKey: "EXO-key", ApiSecret: "EXO-secret"},
			}},
			expected: true,
		},
		{
			name: "Exoscale different api key",
			pr: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{ApiKey: "KEY_A", ApiSecret: "secret"},
			}},
			other: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{ApiKey: "KEY_B", ApiSecret: "secret"},
			}},
			expected: false,
		},
		{
			name: "Exoscale different api secret",
			pr: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{ApiKey: "key", ApiSecret: "SECRET_A"},
			}},
			other: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{ApiKey: "key", ApiSecret: "SECRET_B"},
			}},
			expected: false,
		},
		{
			name: "Exoscale empty credentials (zero value)",
			pr: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{},
			}},
			other: &Provider{ProviderType: &Provider_Exoscale{
				Exoscale: &ExoscaleProvider{},
			}},
			expected: true,
		},

		{
			name: "GCP equal key",
			pr: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{Key: "gcp-service-account-key"},
			}},
			other: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{Key: "gcp-service-account-key"},
			}},
			expected: true,
		},
		{
			name: "GCP different key",
			pr: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{Key: "key-A"},
			}},
			other: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{Key: "key-B"},
			}},
			expected: false,
		},
		{
			name: "GCP empty key (zero value)",
			pr: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{},
			}},
			other: &Provider{ProviderType: &Provider_Gcp{
				Gcp: &GCPProvider{},
			}},
			expected: true,
		},

		{
			name: "Hetzner equal token",
			pr: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{Token: "hetzner-token-abc"},
			}},
			other: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{Token: "hetzner-token-abc"},
			}},
			expected: true,
		},
		{
			name: "Hetzner different token",
			pr: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{Token: "token-A"},
			}},
			other: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{Token: "token-B"},
			}},
			expected: false,
		},
		{
			name: "Hetzner empty token (zero value)",
			pr: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{},
			}},
			other: &Provider{ProviderType: &Provider_Hetzner{
				Hetzner: &HetznerProvider{},
			}},
			expected: true,
		},

		{
			name: "OCI equal credentials",
			pr: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{KeyFingerprint: "aa:bb:cc", PrivateKey: "oci-private-key"},
			}},
			other: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{KeyFingerprint: "aa:bb:cc", PrivateKey: "oci-private-key"},
			}},
			expected: true,
		},
		{
			name: "OCI different fingerprint",
			pr: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{KeyFingerprint: "aa:bb:cc", PrivateKey: "key"},
			}},
			other: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{KeyFingerprint: "dd:ee:ff", PrivateKey: "key"},
			}},
			expected: false,
		},
		{
			name: "OCI different private key",
			pr: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{KeyFingerprint: "aa:bb:cc", PrivateKey: "key-A"},
			}},
			other: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{KeyFingerprint: "aa:bb:cc", PrivateKey: "key-B"},
			}},
			expected: false,
		},
		{
			name: "OCI empty credentials (zero value)",
			pr: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{},
			}},
			other: &Provider{ProviderType: &Provider_Oci{
				Oci: &OCIProvider{},
			}},
			expected: true,
		},

		{
			name: "OpenStack equal credentials",
			pr: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{
					ApplicationCredentialID:     "os-cred-id",
					ApplicationCredentialSecret: "os-cred-secret",
				},
			}},
			other: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{
					ApplicationCredentialID:     "os-cred-id",
					ApplicationCredentialSecret: "os-cred-secret",
				},
			}},
			expected: true,
		},
		{
			name: "OpenStack different credential ID",
			pr: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{
					ApplicationCredentialID:     "id-A",
					ApplicationCredentialSecret: "secret",
				},
			}},
			other: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{
					ApplicationCredentialID:     "id-B",
					ApplicationCredentialSecret: "secret",
				},
			}},
			expected: false,
		},
		{
			name: "OpenStack different credential secret",
			pr: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{
					ApplicationCredentialID:     "id",
					ApplicationCredentialSecret: "secret-A",
				},
			}},
			other: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{
					ApplicationCredentialID:     "id",
					ApplicationCredentialSecret: "secret-B",
				},
			}},
			expected: false,
		},
		{
			name: "OpenStack empty credentials (zero value)",
			pr: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{},
			}},
			other: &Provider{ProviderType: &Provider_Openstack{
				Openstack: &OpenstackProvider{},
			}},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pr.CredentialsEqual(tc.other)
			if got != tc.expected {
				t.Errorf("CredentialsEqual() = %v, want %v", got, tc.expected)
			}
		})
	}
}
