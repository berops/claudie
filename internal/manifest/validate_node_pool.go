package manifest

import (
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/utils"

	"github.com/go-playground/validator/v10"
	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

const TotalAnnotationSizeLimitB int = 256 * (1 << 10) // 256 kB

// Validate validates the parsed data inside the NodePool section of the manifest.
// It checks for missing/invalid filled out values defined in the NodePool section of
// the manifest.
func (p *NodePool) Validate(m *Manifest) error {
	names := make(map[string]bool)

	for _, n := range p.Dynamic {
		if !IsReferenced(n.Name, m) {
			return fmt.Errorf("unused nodepool %q, unused nodepools are not alloved", n.Name)
		}

		// check if the provider is defined in the manifest
		if _, err := m.GetProvider(n.ProviderSpec.Name); err != nil {
			return fmt.Errorf("provider %q specified for DynamicNodePool %q doesn't exists", n.ProviderSpec.Name, n.Name)
		}

		if err := n.Validate(m); err != nil {
			return fmt.Errorf("failed to validate DynamicNodePool %q: %w", n.Name, err)
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

	reusedStaticIp := make(map[string]string)
	for _, n := range p.Static {
		if !IsReferenced(n.Name, m) {
			return fmt.Errorf("unused nodepool %q, unused nodepools are not alloved", n.Name)
		}
		if err := n.Validate(); err != nil {
			return fmt.Errorf("failed to validate StaticNodePool %q: %w", n.Name, err)
		}

		for _, sn := range n.Nodes {
			if otherNodePool, ok := reusedStaticIp[sn.Endpoint]; ok {
				nodepools := utils.RemoveDuplicates([]string{n.Name, otherNodePool})
				return fmt.Errorf("same IP %q is referenced by multiple static nodes inside %q", sn.Endpoint, nodepools)
			}
			reusedStaticIp[sn.Endpoint] = n.Name
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
		if err := checkAnnotations(n.Annotations); err != nil {
			return fmt.Errorf("nodepool %s has incorrectly defined annotations : %w", n.Name, err)
		}
	}

	return nil
}

// IsReferenced checks whether a nodepool is in use. Unused nodepools are considered as an error.
func IsReferenced(name string, m *Manifest) bool {
	for _, k8s := range m.Kubernetes.Clusters {
		for _, control := range k8s.Pools.Control {
			if control == name {
				return true
			}
		}

		for _, compute := range k8s.Pools.Compute {
			if compute == name {
				return true
			}
		}
	}

	for _, lb := range m.LoadBalancer.Clusters {
		for _, np := range lb.Pools {
			if np == name {
				return true
			}
		}
	}

	return false
}

func (d *DynamicNodePool) Validate(m *Manifest) error {
	if (d.StorageDiskSize != nil) && !(*d.StorageDiskSize == 0 || *d.StorageDiskSize >= 50) {
		return fmt.Errorf("storageDiskSize size must be either 0 or >= 50")
	}

	validate := validator.New()
	validate.RegisterStructValidation(func(sl validator.StructLevel) {
		dnp := sl.Current().Interface().(DynamicNodePool)

		found := false
		for _, p := range m.Providers.GenesisCloud {
			if p.Name == dnp.ProviderSpec.Name {
				found = true
			}
		}

		if !found && dnp.ProviderSpec.Zone == "" {
			sl.ReportError(dnp.ProviderSpec.Zone, "Zone", "Zone", "required", "")
		}
	}, DynamicNodePool{})

	return validate.Struct(d)
}

func (s *StaticNodePool) Validate() error { return validator.New().Struct(s) }

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
	for k, v := range labels {
		if errs := validation.IsQualifiedName(k); len(errs) > 0 {
			return fmt.Errorf("key %v is not valid  : %v", k, errs)
		}
		if errs := validation.IsValidLabelValue(v); len(errs) > 0 {
			return fmt.Errorf("value %v is not valid  : %v", v, errs)
		}
	}
	return nil
}

func checkAnnotations(annotations map[string]string) error {
	var totalSize int64
	for k, v := range annotations {
		// The rule is QualifiedName except that case doesn't matter, so convert to lowercase before checking.
		if errs := validation.IsQualifiedName(strings.ToLower(k)); len(errs) > 0 {
			return fmt.Errorf("key %v is not valid  : %v", k, errs)
		}
		totalSize += (int64)(len(k)) + (int64)(len(v))
	}
	if totalSize > (int64)(TotalAnnotationSizeLimitB) {
		return fmt.Errorf("annotations size %d is larger than limit %d", totalSize, TotalAnnotationSizeLimitB)
	}
	return nil
}
