package manifest

import (
	"fmt"
	"slices"

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
		roles = make(map[string]Role)

		// check if hostnames in the same DNS zone are unique
		// There is a edge-case. If an empty string is supplied
		// for the hostname a random hash will be generated.
		// Thus, we only check for non-empty string.
		// https://github.com/berops/claudie/blob/master/docs/input-manifest/input-manifest.md#dns
		hostnamesPerDNS = make(map[string]string)

		// check for cluster name uniqueness.
		clusters = make(map[string]bool)

		// check if the roles in the LB cluster has a role of ApiServer.
		apiServerRole Role
	)

	for _, role := range l.Roles {
		if err := role.Validate(); err != nil {
			return fmt.Errorf("failed to validate role %q: %w", role.Name, err)
		}

		// save the result so we can use it later.
		if role.TargetPort == APIServerPort {
			apiServerRole = role
		}

		if _, ok := roles[role.Name]; ok {
			return fmt.Errorf("name %q is used across multiple roles, must be unique", role.Name)
		}

		targetPoolsDuplicates := make(map[string]bool)

		for _, np := range role.TargetPools {
			if ok, _ := m.nodePoolDefined(np); !ok {
				return fmt.Errorf("role %q targets undefined nodepool %q", role.Name, np)
			}
			if _, ok := targetPoolsDuplicates[np]; ok {
				return fmt.Errorf("role %q has target pool %q referenced more than once. remove duplicates", role.Name, np)
			}
			targetPoolsDuplicates[np] = true
		}
		roles[role.Name] = role
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
			roleDef, ok := roles[role]
			if !ok {
				return fmt.Errorf("role %q used inside cluster %q is not defined", role, cluster.Name)
			}
			// check if the target pools of the role are referencing valid nodepools for the k8s cluster.
			for _, k8s := range m.Kubernetes.Clusters {
				if k8s.Name == cluster.TargetedK8s {
					for _, np := range roleDef.TargetPools {
						var found bool
						for _, nnp := range k8s.Pools.Control {
							found = found || np == nnp
						}
						for _, nnp := range k8s.Pools.Compute {
							found = found || np == nnp
						}
						if !found {
							return fmt.Errorf("role definition for %q used for %q targets nodepool %q which is not used by the target k8s cluster %q", roleDef.Name, cluster.Name, np, cluster.TargetedK8s)
						}
					}
				}
			}

			if role == apiServerRole.Name {
				// check if this is an ApiServer LB and another ApiServer LB already exists.
				if apiServerLBExists[cluster.TargetedK8s] {
					return fmt.Errorf("role %q is used across multiple Load-Balancers for k8s-cluster %s. Can have only one ApiServer Load-Balancer per k8s-cluster", role, cluster.TargetedK8s)
				}

				// this is the first LB that uses the ApiServer role.
				apiServerLBExists[cluster.TargetedK8s] = true

				// check if the target nodepools are referencing control nodepools.
				for _, k8s := range m.Kubernetes.Clusters {
					if k8s.Name == cluster.TargetedK8s {
						for _, np := range apiServerRole.TargetPools {
							var found bool
							for _, cnp := range k8s.Pools.Control {
								found = found || np == cnp
							}
							if !found {
								return fmt.Errorf("api server role %q used for %q must only target control nodepools, %q is not a control nodepool of kubernetes cluster %q", apiServerRole.Name, cluster.Name, np, cluster.TargetedK8s)
							}
						}
					}
				}
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

		if !slices.Contains([]string{"gcp", "aws", "azure", "oci", "cloudflare", "hetznerdns"}, provider.CloudProviderName) {
			return fmt.Errorf("provider %q used inside cluster %q exists but is not a supported provider", cluster.DNS.Provider, cluster.Name)
		}

		// check if k8s cluster was defined in the manifest.
		if !m.IsKubernetesClusterPresent(cluster.TargetedK8s) {
			return fmt.Errorf("target k8s %q used inside cluster %q is not defined", cluster.TargetedK8s, cluster.Name)
		}

		// check for nodepool uniqueness.
		poolNames := make(map[string]bool)
		// check if requested pools are defined
		for _, pool := range cluster.Pools {
			defined, static := m.nodePoolDefined(pool)
			if !defined {
				return fmt.Errorf("nodepool %q used inside cluster %q is not defined", pool, cluster.Name)
			}

			if _, ok := poolNames[pool]; ok {
				if !static {
					return fmt.Errorf("nodepool %q used multiple times as a loadbalancer nodepool, this effect can be achieved by increasing the \"count\" field or defining a new nodepool with a different name", pool)
				}
				return fmt.Errorf("static nodepool %q used multiple times as loadbalancer nodepool, reusing the same static nodepool is discouraged as it can introduce issues within the cluster. Make sure to use a different static nodepool", pool)
			}
			poolNames[pool] = true
		}
	}

	if err := validateLBStaticNodePool(m, l.Clusters); err != nil {
		return fmt.Errorf("failed to validate static nodepools: %w", err)
	}

	return nil
}

func (r *Role) Validate() error {
	if err := validator.New().Struct(r); err != nil {
		return prettyPrintValidationError(err)
	}
	return nil
}

func (c *LoadBalancerCluster) Validate() error {
	if err := validator.New().Struct(c); err != nil {
		return prettyPrintValidationError(err)
	}
	return nil
}

func validateLBStaticNodePool(m *Manifest, clusters []LoadBalancerCluster) error {
	for _, cluster := range clusters {
		for _, pool := range cluster.Pools {
			if _, static := m.nodePoolDefined(pool); !static {
				continue
			}

			// err is checked in the validate k8s part.
			m, _ := validateStaticNodepool(m, m.Kubernetes.Clusters)

			// check if the static nodepool in the LB cluster is used as a control or compute nodepool.
			if clstr, ok := m[pool]; ok {
				clusters := []string{cluster.Name, clstr}
				return fmt.Errorf("static nodepool %q used multiple times as a loadbalancer and as a control/compute plane nodepool within %q, reusing the same static nodepool is discouraged as it can introduce issues within the cluster. Make sure to use a different static nodepool", pool, clusters)
			}
		}
	}
	return nil
}
