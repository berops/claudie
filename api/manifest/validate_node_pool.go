package manifest

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/berops/claudie/internal/generics"

	"github.com/go-playground/validator/v10"
	k8sV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

const TotalAnnotationSizeLimitB int = 256 * (1 << 10) // 256 kB

var (
	controlPlaneDeniedProviders = []string{"vastai"}
	loadBalancerDeniedProviders = []string{"vastai"}
	minStorageDiskSize          = map[string]int32{"vastai": 130}
)

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
		if err := checkAnnotations(n.Annotations); err != nil {
			return fmt.Errorf("nodepool %s has incorrectly defined annotations: %w", n.Name, err)
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
		if slices.Contains(k8s.Pools.Control, name) {
			return true
		}

		if slices.Contains(k8s.Pools.Compute, name) {
			return true
		}
	}

	for _, lb := range m.LoadBalancer.Clusters {
		if slices.Contains(lb.Pools, name) {
			return true
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

	// Validate provider specific minimum storageDiskSize
	if err := d.validateStorageDiskSize(m); err != nil {
		return err
	}

	// Validate Spot instance constraints
	if err := d.validateSpot(m); err != nil {
		return err
	}

	// Validate the provider is allowed to back control-plane nodepools
	if err := d.validateControlPlane(m); err != nil {
		return err
	}

	// Validate the provider is allowed to back load balancer nodepools
	if err := d.validateLoadBalancer(m); err != nil {
		return err
	}

	// Validate provider specific machineSpec requirements
	if err := d.validateMachineSpec(m); err != nil {
		return err
	}

	// Validate OpenStack external network requirement
	if err := d.validateExternalNet(m); err != nil {
		return err
	}

	if err := validator.New().Struct(d); err != nil {
		return prettyPrintValidationError(err)
	}
	return nil
}

// isControlPlane reports whether a nodepool name appears in any cluster's control-plane pool list.
func isControlPlane(name string, m *Manifest) bool {
	for _, k8s := range m.Kubernetes.Clusters {
		if slices.Contains(k8s.Pools.Control, name) {
			return true
		}
	}
	return false
}

// isLoadBalancer reports whether a nodepool name appears in any load balancer cluster's pool list.
func isLoadBalancer(name string, m *Manifest) bool {
	for _, lb := range m.LoadBalancer.Clusters {
		if slices.Contains(lb.Pools, name) {
			return true
		}
	}
	return false
}

// validateStorageDiskSize requires an explicit storageDiskSize of at least the minimum
// listed in minStorageDiskSize for the referenced provider type.
func (d *DynamicNodePool) validateStorageDiskSize(m *Manifest) error {
	providerType, err := m.GetProviderType(d.ProviderSpec.Name)
	if err != nil {
		// Provider existence is validated in [NodePool.Validate] before
		// calling [DynamicNodePool.Validate].
		return nil
	}

	minSize, ok := minStorageDiskSize[providerType]
	if !ok {
		return nil
	}

	if d.StorageDiskSize == nil || *d.StorageDiskSize < minSize {
		return fmt.Errorf("storageDiskSize of at least %d is required for provider type %q", minSize, providerType)
	}

	return nil
}

// validateSpot checks that spot instances are only requested on supported worker pools.
func (d *DynamicNodePool) validateSpot(m *Manifest) error {
	if !d.Spot {
		return nil
	}

	providerType, err := m.GetProviderType(d.ProviderSpec.Name)
	if err != nil {
		// Provider existence is validated in [NodePool.Validate] before
		// calling [DynamicNodePool.Validate].
		return nil
	}

	switch providerType {
	case "gcp", "verda", "aws", "azure", "oci":
	default:
		return fmt.Errorf("spot instances are only supported on GCP, Verda, AWS, Azure, OCI; provider %q has type %q", d.ProviderSpec.Name, providerType)
	}

	if isControlPlane(d.Name, m) {
		return fmt.Errorf("spot instances are not allowed on control-plane nodepools (etcd data-corruption hazard)")
	}

	return nil
}

// validateControlPlane checks that the nodepool's provider is allowed to back a control-plane nodepool.
func (d *DynamicNodePool) validateControlPlane(m *Manifest) error {
	if !isControlPlane(d.Name, m) {
		return nil
	}

	providerType, err := m.GetProviderType(d.ProviderSpec.Name)
	if err != nil {
		// Provider existence is validated in [NodePool.Validate] before
		// calling [DynamicNodePool.Validate].
		return nil
	}

	if slices.Contains(controlPlaneDeniedProviders, providerType) {
		return fmt.Errorf("provider type %q cannot be used for control-plane nodepools, nodepool %q is used as a control-plane nodepool", providerType, d.Name)
	}

	return nil
}

// validateLoadBalancer checks that the nodepool's provider is allowed to back a load balancer nodepool.
func (d *DynamicNodePool) validateLoadBalancer(m *Manifest) error {
	if !isLoadBalancer(d.Name, m) {
		return nil
	}

	providerType, err := m.GetProviderType(d.ProviderSpec.Name)
	if err != nil {
		// Provider existence is validated in [NodePool.Validate] before
		// calling [DynamicNodePool.Validate].
		return nil
	}

	if slices.Contains(loadBalancerDeniedProviders, providerType) {
		return fmt.Errorf("provider type %q cannot be used for load balancer nodepools, nodepool %q is used as a load balancer nodepool", providerType, d.Name)
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

// validateMachineSpec validates the machineSpec against the provider specific requirements.
func (d *DynamicNodePool) validateMachineSpec(m *Manifest) error {
	providerType, err := m.GetProviderType(d.ProviderSpec.Name)
	if err != nil {
		// Provider existence is validated in [NodePool.Validate] before
		// calling [DynamicNodePool.Validate].
		return nil
	}

	spec := d.MachineSpec

	switch providerType {
	case "vastai":
		// machineSpec with both nvidiaGpuType and nvidiaGpuCount is mandatory.
		if spec == nil {
			return fmt.Errorf("machineSpec is required for VastAI provider")
		}

		// Check both NvidiaGpuCount (new) and NvidiaGpu (deprecated) for backward compatibility
		if spec.NvidiaGpuType == "" || (spec.NvidiaGpuCount == 0 && spec.NvidiaGpu == 0) {
			return fmt.Errorf("machineSpec.nvidiaGpuType and machineSpec.nvidiaGpuCount are required for VastAI provider")
		}

	case "gcp":
		// machineSpec is optional. When provided it must specify either cpuCount/memory
		// or both nvidiaGpuType and nvidiaGpuCount, as GCP attaches GPUs only with both.
		if spec == nil {
			return nil
		}

		// Check both NvidiaGpuCount (new) and NvidiaGpu (deprecated) for backward compatibility
		hasGpuCount := spec.NvidiaGpuCount > 0 || spec.NvidiaGpu > 0
		hasGpuType := spec.NvidiaGpuType != ""

		if hasGpuCount != hasGpuType {
			return fmt.Errorf("machineSpec.nvidiaGpuType and machineSpec.nvidiaGpuCount must be specified together for GCP provider")
		}
	}

	return nil
}

// validateExternalNet requires externalNetworkName when the referenced provider type is openstack.
func (d *DynamicNodePool) validateExternalNet(m *Manifest) error {
	providerType, err := m.GetProviderType(d.ProviderSpec.Name)
	if err != nil {
		// Provider existence is validated in [NodePool.Validate] before
		// calling [DynamicNodePool.Validate].
		return nil
	}

	if providerType != "openstack" {
		return nil
	}

	if d.ProviderSpec.ExternalNetworkName == "" {
		return fmt.Errorf("externalNetworkName is required for OpenStack provider")
	}

	return nil
}
