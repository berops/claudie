package manifest

import (
	"fmt"
	"math"
	"strings"

	"github.com/berops/claudie/internal/generics"

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
			return fmt.Errorf("unused nodepool %q, unused nodepools are not allowed", n.Name)
		}

		// check if the provider is defined in the manifest
		if _, err := m.GetProviderType(n.ProviderSpec.Name); err != nil {
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
			return fmt.Errorf("unused nodepool %q, unused nodepools are not allowed", n.Name)
		}
		if err := n.Validate(); err != nil {
			return fmt.Errorf("failed to validate StaticNodePool %q: %w", n.Name, err)
		}

		for _, sn := range n.Nodes {
			if otherNodePool, ok := reusedStaticIp[sn.Endpoint]; ok {
				nodepools := generics.RemoveDuplicates([]string{n.Name, otherNodePool})
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
	//nolint
	if (d.StorageDiskSize != nil) && !(*d.StorageDiskSize == 0 || *d.StorageDiskSize >= 50) {
		return fmt.Errorf("storageDiskSize size must be either 0 or >= 50")
	}

	if d.Count > math.MaxUint8 {
		return fmt.Errorf("max available count for a nodepool is 255")
	}

	// Validate GCP-specific GPU configuration
	if err := d.validateGCPGpuConfig(m); err != nil {
		return err
	}

	validate := validator.New()

	if err := validate.RegisterValidation("external_net", validateExternalNet); err != nil {
		return err
	}

	if err := validate.Struct(d); err != nil {
		return prettyPrintValidationError(err)
	}
	return nil
}

// validateGCPGpuConfig validates that GCP nodepools with GPUs have the required type specified.
// GCP requires the guest_accelerator block with both type and count to attach GPUs to instances.
func (d *DynamicNodePool) validateGCPGpuConfig(m *Manifest) error {
	providerType, err := m.GetProviderType(d.ProviderSpec.Name)
	if err != nil {
		return err
	}

	if providerType != "gcp" {
		return nil
	}

	if d.MachineSpec == nil {
		return nil
	}

	// Check both NvidiaGpuCount (new) and NvidiaGpu (deprecated) for backward compatibility
	gpuCount := d.MachineSpec.NvidiaGpuCount
	if gpuCount == 0 {
		gpuCount = d.MachineSpec.NvidiaGpu
	}

	if gpuCount > 0 && d.MachineSpec.NvidiaGpuType == "" {
		return fmt.Errorf("nvidiaGpuType is required for GCP when nvidiaGpuCount > 0")
	}

	return nil
}

func (s *StaticNodePool) Validate() error {
	if err := validator.New().Struct(s); err != nil {
		return prettyPrintValidationError(err)
	}
	return nil
}

func (a *AutoscalerConfig) isDefined() bool { return a.Min >= 0 && a.Max > 0 }

func checkTaints(taints []k8sV1.Taint) error {
	for _, t := range taints {
		// Check if effect is supported
		// nolint
		if !(t.Effect == k8sV1.TaintEffectNoSchedule || t.Effect == k8sV1.TaintEffectNoExecute || t.Effect == k8sV1.TaintEffectPreferNoSchedule) {
			return fmt.Errorf("taint effect \"%s\" is not supported", t.Effect)
		}

		// taints are validated similarly to labels
		// https://github.com/kubernetes/kubectl/blob/8185d35b7a2cd69d364f0f09648ecdd94c9fb5b7/pkg/cmd/taint/utils.go#L101
		// https://github.com/kubernetes/kubectl/blob/8185d35b7a2cd69d364f0f09648ecdd94c9fb5b7/pkg/cmd/taint/utils.go#L109
		if errs := validation.IsQualifiedName(t.Key); len(errs) > 0 {
			return fmt.Errorf("invalid taint key %v: %v", t.Key, errs)
		}
		if errs := validation.IsValidLabelValue(t.Value); len(errs) > 0 {
			return fmt.Errorf("value %v is not valid: %v", t.Value, errs)
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

func validateExternalNet(fl validator.FieldLevel) bool {
	providerSpec := fl.Parent().Interface().(ProviderSpec)
	if providerSpec.Name == "openstack" {
		return providerSpec.ExternalNetworkName != ""
	}
	return true
}
