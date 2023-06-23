package manifest

import (
	"fmt"
	"regexp"

	"github.com/go-playground/validator/v10"
	k8sV1 "k8s.io/api/core/v1"
)

const (
	labelRegex = "^(?:[a-z0-9A-Z](?:[-_.]?[a-z0-9A-Z]){0,61})?(?:\\.[a-z0-9A-Z](?:[-_.]?[a-z0-9A-Z]){0,61})*(?:\\/[a-z0-9A-Z](?:[-_.]?[a-z0-9A-Z]){0,61})?$"
)

// Validate validates the parsed data inside the NodePool section of the manifest.
// It checks for missing/invalid filled out values defined in the NodePool section of
// the manifest.
func (p *NodePool) Validate(m *Manifest) error {
	// check for name uniqueness across node pools.
	names := make(map[string]bool)

	for _, n := range p.Dynamic {
		if err := n.Validate(); err != nil {
			return fmt.Errorf("failed to validate DynamicNodePool %q: %w", n.Name, err)
		}

		// check if the provider is defined in the manifest
		if _, err := m.GetProvider(n.ProviderSpec.Name); err != nil {
			return fmt.Errorf("provider %q specified for DynamicNodePool %q doesn't exists", n.ProviderSpec.Name, n.Name)
		}

		// check if the name is already used by a different node pool
		if _, ok := names[n.Name]; ok {
			return fmt.Errorf("name %q is used across multiple node pools, must be unique", n.Name)
		}
		names[n.Name] = true

		// check if the count and autoscaler are mutually exclusive
		if n.Count != 0 && n.AutoscalerConfig.isDefined() {
			return fmt.Errorf("nodepool %s cannot have both, autoscaler enabled and \"count\" defined", n.Name)
		}
		if err := checkTaints(n.Taints); err != nil {
			return fmt.Errorf("nodepool %s has incorrectly defined taints : %w", n.Name, err)
		}
		if err := checkLabels(n.Labels); err != nil {
			return fmt.Errorf("nodepool %s has incorrectly defined labels : %w", n.Name, err)
		}
	}

	for _, n := range p.Static {
		if err := n.Validate(); err != nil {
			return fmt.Errorf("failed to validate StaticNodePool %q: %w", n.Name, err)
		}

		// check if the name is already used by a different node pool
		if _, ok := names[n.Name]; ok {
			return fmt.Errorf("name %q is used across multiple node pools, must be unique", n.Name)
		}
		names[n.Name] = true
		if err := checkTaints(n.Taints); err != nil {
			return fmt.Errorf("nodepool %s has incorrectly defined taints : %w", n.Name, err)
		}
		if err := checkLabels(n.Labels); err != nil {
			return fmt.Errorf("nodepool %s has incorrectly defined labels : %w", n.Name, err)
		}
	}

	return nil
}

func (d *DynamicNodePool) Validate() error { return validator.New().Struct(d) }
func (s *StaticNodePool) Validate() error  { return validator.New().Struct(s) }

func (a *AutoscalerConfig) isDefined() bool { return a.Min >= 0 && a.Max > 0 }

func checkTaints(taints []k8sV1.Taint) error {
	for _, t := range taints {
		// Check if effect is supported
		if !(t.Effect == k8sV1.TaintEffectNoSchedule || t.Effect == k8sV1.TaintEffectNoExecute || t.Effect == k8sV1.TaintEffectPreferNoSchedule) {
			return fmt.Errorf("taint effect \"%s\" is not supported", t.Effect)
		}
	}
	return nil
}

func checkLabels(labels map[string]string) error {
	r := regexp.MustCompile(labelRegex)
	for k, v := range labels {
		if ok := r.MatchString(k); !ok {
			return fmt.Errorf("key %s is not a valid key for label", k)
		}
		if ok := r.MatchString(v); !ok {
			return fmt.Errorf("value %s is not a valid value for label", v)
		}
	}
	return nil
}
