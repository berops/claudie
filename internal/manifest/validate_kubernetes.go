package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var (
	// semverRegexString is a regex expression used for parsing semantic versioning
	// based on https://github.com/go-playground/validator/blob/master/regexes.go#L65:2
	// NOTE:
	// first/second capturing group MUST be changed whenever new kubeone version is introduced in Claudie
	// so validation will catch unsupported versions
	semverRegexString = `^(1)\.(22|23|24)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`

	// semverRegex is a regex using the semverRegexString.
	// It's used to verify the version inside the manifest,
	// as kubernetes follows the semantic version terminology
	// https://kubernetes.io/releases/
	semverRegex = regexp.MustCompile(semverRegexString)
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

		for _, pool := range cluster.Pools.Control {
			if p := m.FindNodePool(pool); p == nil {
				return fmt.Errorf("control nodepool %q used inside cluster %q not defined inside manifest", pool, cluster.Name)
			}
		}

		for _, pool := range cluster.Pools.Compute {
			if p := m.FindNodePool(pool); p == nil {
				return fmt.Errorf("compute nodepool %q used inside cluster %q not defined inside manifest", pool, cluster.Name)
			}
		}
	}

	return nil
}

func (c *Cluster) Validate() error {
	validate := validator.New()

	// register custom validation function to validate the kubernetes version.
	if err := validate.RegisterValidation("ver", validateVersion, false); err != nil {
		return err
	}

	return validate.Struct(c)
}

func validateVersion(fl validator.FieldLevel) bool {
	semverString := fl.Field().String()

	// drop the 'v' as it's not part of a semantic version (https://semver.org/)
	semverString = strings.TrimPrefix(semverString, "v")

	return semverRegex.MatchString(semverString)
}
