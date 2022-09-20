package manifest

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// Validate validates the parsed data inside the LoadBalancer section of the manifest.
// It checks for missing/invalid filled out values defined in the LoadBalancer section
// of the manifest.
func (l *LoadBalancer) Validate(m *Manifest) error {
	roles := make(map[string]bool)

	for _, role := range l.Roles {
		if err := role.Validate(); err != nil {
			return fmt.Errorf("failed to validate role %q: %w", role.Name, err)
		}

		roles[role.Name] = true
	}

	for _, cluster := range l.Clusters {
		if err := cluster.Validate(); err != nil {
			return fmt.Errorf("failed to validate cluster %q: %w", cluster.Name, err)
		}

		// check if requested roles are defined.
		for _, role := range cluster.Roles {
			if _, ok := roles[role]; !ok {
				return fmt.Errorf("role %q used inside cluster %q is not defined", role, cluster.Name)
			}
		}

		// check if the requested provider for the DNS zone
		// is defined in the manifest and if it's a GCP provider.
		// https://github.com/Berops/claudie/blob/master/docs/input-manifest/input-manifest.md#dns
		provider, err := m.GetProvider(cluster.DNS.Provider)
		if err != nil {
			return fmt.Errorf("provider %q used inside cluster %q is not defined", cluster.DNS.Provider, cluster.Name)
		}

		if provider.CloudProviderName != "gcp" {
			return fmt.Errorf("provider %q used inside cluster %q exists but is not GCP (Google Cloud Platform)", cluster.DNS.Provider, cluster.Name)
		}

		// check if k8s cluster was defined in the manifest.
		if !m.IsKubernetesClusterPresent(cluster.TargetedK8s) {
			return fmt.Errorf("target k8s %q used inside cluster %q is not defined", cluster.TargetedK8s, cluster.Name)
		}

		// check if requested pools are defined
		for _, pool := range cluster.Pools {
			if p := m.FindNodePool(pool); p == nil {
				return fmt.Errorf("nodepool %q used inside cluster %q is not defined", pool, cluster.Name)
			}
		}
	}

	return nil
}

func (r *Role) Validate() error                { return validator.New().Struct(r) }
func (c *LoadBalancerCluster) Validate() error { return validator.New().Struct(c) }
