package manifest

import (
	"fmt"

	"github.com/berops/claudie/internal/templateUtils"

	"github.com/go-playground/validator/v10"
)

// APIServerPort is the port on which the ApiServer listens.
const APIServerPort = 6443

// Validate validates the parsed data inside the LoadBalancer section of the manifest.
// It checks for missing/invalid filled out values defined in the LoadBalancer section
// of the manifest.
func (l *LoadBalancer) Validate(m *Manifest) error {
	var (
		// check if requested roles exists and has a unique name.
		roles = make(map[string]bool)

		// check if hostnames in the same DNS zone are unique
		// There is a edge-case. If an empty string is supplied
		// for the hostname a random hash will be generated.
		// Thus, we only check for non-empty string.
		// https://github.com/berops/claudie/blob/master/docs/input-manifest/input-manifest.md#dns
		hostnamesPerDNS = make(map[string]string)

		// check for cluster name uniqueness.
		clusters = make(map[string]bool)

		// check if the roles in the LB cluster has a role of ApiServer.
		apiServerRole = ""
	)

	for _, role := range l.Roles {
		if err := role.Validate(); err != nil {
			return fmt.Errorf("failed to validate role %q: %w", role.Name, err)
		}

		// save the result so we can use it later.
		if role.TargetPort == APIServerPort {
			apiServerRole = role.Name
		}

		if _, ok := roles[role.Name]; ok {
			return fmt.Errorf("name %q is used across multiple roles, must be unique", role.Name)
		}
		roles[role.Name] = true
	}

	apiServerLBExists := make(map[string]bool) // [Targetk8sClusterName]bool
	for _, cluster := range l.Clusters {
		// check if the name used for the cluster is unique
		if _, ok := clusters[cluster.Name]; ok {
			return fmt.Errorf("name %q is used across multiple clusters, must be unique", cluster.Name)
		}

		clusters[cluster.Name] = true

		if err := cluster.Validate(); err != nil {
			return fmt.Errorf("failed to validate cluster %q: %w", cluster.Name, err)
		}

		// check if requested roles are defined.
		for _, role := range cluster.Roles {
			if _, ok := roles[role]; !ok {
				return fmt.Errorf("role %q used inside cluster %q is not defined", role, cluster.Name)
			}

			if role == apiServerRole {
				// check if this is an ApiServer LB and another ApiServer LB already exists.
				if apiServerLBExists[cluster.TargetedK8s] {
					return fmt.Errorf("role %q is used across multiple Load-Balancers for k8s-cluster %s. Can have only one ApiServer Load-Balancer per k8s-cluster", role, cluster.TargetedK8s)
				}

				// this is the first LB that uses the ApiServer role.
				apiServerLBExists[cluster.TargetedK8s] = true
			}
		}

		// check if the requested hostname is unique per DNS-ZONE
		if zone, ok := hostnamesPerDNS[cluster.DNS.Hostname]; ok && zone == cluster.DNS.DNSZone {
			return fmt.Errorf("hostname %q used in cluster %q is used across multiple clusters for the same DNS zone %q, must be unique", cluster.DNS.Hostname, cluster.Name, zone)
		}

		if cluster.DNS.Hostname != "" {
			hostnamesPerDNS[cluster.DNS.Hostname] = cluster.DNS.DNSZone
		}

		// check if the requested provider for the DNS zone
		// is defined in the manifest and if it's a GCP provider.
		// https://github.com/berops/claudie/blob/master/docs/input-manifest/input-manifest.md#dns
		provider, err := m.GetProvider(cluster.DNS.Provider)
		if err != nil {
			return fmt.Errorf("provider %q used inside cluster %q is not defined", cluster.DNS.Provider, cluster.Name)
		}

		if templateUtils.IsMissing(provider.CloudProviderName, []string{"gcp", "aws", "azure", "oci", "cloudflare", "hetznerdns"}) {
			return fmt.Errorf("provider %q used inside cluster %q exists but is not a supported provider", cluster.DNS.Provider, cluster.Name)
		}

		// check if k8s cluster was defined in the manifest.
		if !m.IsKubernetesClusterPresent(cluster.TargetedK8s) {
			return fmt.Errorf("target k8s %q used inside cluster %q is not defined", cluster.TargetedK8s, cluster.Name)
		}

		// check if requested pools are defined
		for _, pool := range cluster.Pools {
			if m.nodePoolDefined(pool) {
				return fmt.Errorf("nodepool %q used inside cluster %q is not defined", pool, cluster.Name)
			}
		}
	}
	return nil
}

func (r *Role) Validate() error                { return validator.New().Struct(r) }
func (c *LoadBalancerCluster) Validate() error { return validator.New().Struct(c) }
