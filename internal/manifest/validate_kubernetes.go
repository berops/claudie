package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var (
	// kubernetesVersionRegexString is a regex expression used for parsing semantic versioning
	// based on https://github.com/go-playground/validator/blob/master/regexes.go#L65:2
	// NOTE:
	// first/second capturing group MUST be changed whenever new kubeone version is introduced in Claudie
	// so validation will catch unsupported versions
	kubernetesVersionRegexString = `^(1)\.(27|28|29|30)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`

	// semverRegex is a regex using the semverRegexString.
	// It's used to verify the version inside the manifest,
	// as kubernetes follows the semantic version terminology
	// https://kubernetes.io/releases/
	kubernetesVersionRegex = regexp.MustCompile(kubernetesVersionRegexString)

	// proxyModeRegexString is a regex expression used for parsing the proxy mode from the manifest
	// must be changed whenever there's a change in supporting modes
	proxyModeRegexString = `^(on|off|default)$`

	// used to verify the proxy mode inside the manifest
	proxyModeRegex = regexp.MustCompile(proxyModeRegexString)
)

// Validate validates the parsed data inside the Kubernetes section of the manifest.
// It checks for missing/invalid filled out values defined in the Kubernetes section
// of the manifest.
func (k *Kubernetes) Validate(m *Manifest) error {
	// check for name uniqueness across clusters.
	names := make(map[string]bool)

	for _, cluster := range k.Clusters {
		if err := cluster.Validate(); err != nil {
			return fmt.Errorf("failed to validate kubernetes cluster %s: %w", cluster.Name, err)
		}

		// check if the name is already used by a different cluster
		if _, ok := names[cluster.Name]; ok {
			return fmt.Errorf("name %q is used across multiple clusters, must be unique", cluster.Name)
		}
		names[cluster.Name] = true

		if err := validateNodepools(m, &cluster); err != nil {
			return fmt.Errorf("failed to validate nodepools: %w", err)
		}
	}

	if _, err := validateStaticNodepool(m, k.Clusters); err != nil {
		return fmt.Errorf("failed to validate static nodepools: %w", err)
	}

	return nil
}

func (c *Cluster) Validate() error {
	validate := validator.New()

	// register custom validation function to validate the kubernetes version.
	if err := validate.RegisterValidation("ver", validateVersion, false); err != nil {
		return err
	}

	if err := validate.RegisterValidation("proxyMode", validateProxyMode, true); err != nil {
		return err
	}

	if err := validate.Struct(c); err != nil {
		return prettyPrintValidationError(err)
	}
	return nil
}

func validateVersion(fl validator.FieldLevel) bool {
	semverString := fl.Field().String()

	// drop the 'v' as it's not part of a semantic version (https://semver.org/)
	semverString = strings.TrimPrefix(semverString, "v")

	return kubernetesVersionRegex.MatchString(semverString)
}

func validateProxyMode(fl validator.FieldLevel) bool {
	mode := fl.Field().String()
	return proxyModeRegex.MatchString(mode)
}

func validateNodepools(m *Manifest, cluster *Cluster) error {
	// check for re-use of the same nodepool
	computeNames := make(map[string]bool)
	controlNames := make(map[string]bool)

	for _, pool := range cluster.Pools.Control {
		defined, static := m.nodePoolDefined(pool)
		if !defined {
			return fmt.Errorf("control nodepool %q used inside cluster %q not defined inside manifest", pool, cluster.Name)
		}

		if _, ok := controlNames[pool]; ok {
			if !static {
				return fmt.Errorf("nodepool %q used multiple times as control nodepool, this effect can be achieved by increasing the \"count\" field, adjusting the \"autoscaler\" field or defining a new nodepool with a different name", pool)
			}
			return fmt.Errorf("static nodepool %q used multiple times as control nodepool, reusing the same static nodepool is discouraged as it can introduce issues within the cluster. Make sure to use a different static nodepool", pool)
		}
		controlNames[pool] = true
	}

	for _, pool := range cluster.Pools.Compute {
		defined, static := m.nodePoolDefined(pool)
		if !defined {
			return fmt.Errorf("compute nodepool %q used inside cluster %q not defined inside manifest", pool, cluster.Name)
		}

		if _, ok := computeNames[pool]; ok {
			if !static {
				return fmt.Errorf("nodepool %q used multiple times as compute nodepool, this effect can be achieved by increasing the \"count\" field, adjusting the \"autoscaler\" field or defining a new nodepool with a different name", pool)
			}
			return fmt.Errorf("static nodepool %q used multiple times as control nodepool, reusing the same static nodepool is discouraged as it can introduce issues within the cluster. Make sure to use a different static nodepool", pool)
		}
		computeNames[pool] = true

		if static {
			if _, ok := controlNames[pool]; ok {
				return fmt.Errorf("static nodepool %q used multiple times across control and compute planes, reusing the same static nodepool is discouraged as it can introduce issues within the cluster. Make sure to use a different static nodepool", pool)
			}
		}
	}

	return nil
}

// collects a map of static nodepools for each cluster. If a cluster is used across clusters returns an error.
func validateStaticNodepool(m *Manifest, clusters []Cluster) (map[string]string, error) {
	reusedStaticPools := make(map[string]string)

	for _, cluster := range clusters {
		for _, pool := range cluster.Pools.Control {
			if _, static := m.nodePoolDefined(pool); !static {
				continue
			}
			// is reused across clusters of the same config.
			if clstr, ok := reusedStaticPools[pool]; ok && clstr != cluster.Name {
				clusters := []string{cluster.Name, clstr}
				return nil, fmt.Errorf("static nodepool %q used multiple times across clusters %q, reusing the same static nodepool is discouraged as it can introduce issues within the cluster. Make sure to use a different static nodepool", pool, clusters)
			}
			reusedStaticPools[pool] = cluster.Name
		}

		for _, pool := range cluster.Pools.Compute {
			if _, static := m.nodePoolDefined(pool); !static {
				continue
			}
			// is reused across clusters of the same config.
			if clstr, ok := reusedStaticPools[pool]; ok && clstr != cluster.Name {
				clusters := []string{cluster.Name, clstr}
				return nil, fmt.Errorf("static nodepool %q used multiple times across clusters %q, reusing the same static nodepool is discouraged as it can introduce issues within the cluster. Make sure to use a different static nodepool", pool, clusters)
			}
			reusedStaticPools[pool] = cluster.Name
		}
	}

	return reusedStaticPools, nil
}
