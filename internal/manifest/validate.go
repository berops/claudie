package manifest

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// Validate validates the parsed manifest data.
func (m *Manifest) Validate() error {
	if err := validator.New().Struct(m); err != nil {
		return fmt.Errorf("failed to validate manifest: %w", err)
	}

	// Check if at least one provider is defined
	// https://github.com/berops/claudie/blob/master/docs/input-manifest/input-manifest.md#providers
	providers := len(m.Providers.GCP) + len(m.Providers.Hetzner) + len(m.Providers.AWS) +
		len(m.Providers.Azure) + len(m.Providers.OCI) + len(m.Providers.Cloudflare) +
		len(m.Providers.GenesisCloud) + len(m.Providers.HetznerDNS)
	if providers < 1 {
		// Return error only if at least one dynamic nodepool defined.
		if len(m.NodePools.Dynamic) > 0 {
			return fmt.Errorf("need to define at least one provider inside the providers section of the manifest used for dynamic nodepools")
		}
	}

	if err := m.NodePools.Validate(m); err != nil {
		return fmt.Errorf("failed to validate nodepools section inside manifest: %w", err)
	}

	if err := m.Kubernetes.Validate(m); err != nil {
		return fmt.Errorf("failed to validate kubernetes section inside manifest: %w", err)
	}

	if err := m.LoadBalancer.Validate(m); err != nil {
		return fmt.Errorf("failed to validate loadbalancers section inside manifest: %w", err)
	}

	if err := CheckLengthOfFutureDomain(m); err != nil {
		return fmt.Errorf("failed to validate future domains: %w", err)
	}

	return nil
}
