package controller

import (
	"context"
	"fmt"

	v1beta "github.com/berops/claudie/internal/api/crd/inputmanifest/v1beta1"
	"github.com/berops/claudie/internal/api/manifest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	wbhk "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
)

// InputManifestValidator validates InputManifest containing the input-manifest
type InputManifestValidator struct {
	Logger logr.Logger
	kc     client.Client
}

// NewWebhook returns a new validation webhook for InputManifest resource
func NewWebhook(
	kc client.Client,
	scheme *runtime.Scheme,
	port int,
	dir,
	path string,
	log logr.Logger,
) wbhk.Server {
	hookServer := wbhk.NewServer(wbhk.Options{
		Port:    port,
		CertDir: dir,
	})

	hookServer.Register(path, admission.WithCustomValidator(
		scheme,
		&v1beta.InputManifest{},
		&InputManifestValidator{log, kc},
	))

	return hookServer
}

// validate takes the context and a kubernetes object as a parameter.
// It will extract the secret data out of the received obj and run manifest validation against it
func (v *InputManifestValidator) validate(ctx context.Context, obj runtime.Object) error {
	log := crlog.FromContext(ctx).WithName("InputManifest Validator")

	inputManifest, ok := obj.(*v1beta.InputManifest)
	if !ok {
		return fmt.Errorf("expected an InputManifest but got a %T", obj)
	}

	log.Info("Validating InputManifest")

	if err := validateInputManifest(inputManifest); err != nil {
		log.Error(err, "error validating InputManifest")
		return err
	}

	return nil
}

// ValidateCreate defines the logic when a kubernetes obj resource is created
func (v *InputManifestValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, v.validate(ctx, obj)
}

// ValidateUpdate defines the logic when a kubernetes obj resource is updated
func (v *InputManifestValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	o, ok := oldObj.(*v1beta.InputManifest)
	if !ok {
		return nil, fmt.Errorf("expected InputManifest for 'oldObj' but got %T", oldObj)
	}

	n, ok := newObj.(*v1beta.InputManifest)
	if !ok {
		return nil, fmt.Errorf("expected InputManifest for 'newOjb' but got %T", newObj)
	}

	nmap := getDynamicNodepoolsMap(o)
	for _, desired := range n.Spec.NodePools.Dynamic {
		current, exists := nmap[desired.Name]
		if !exists {
			continue
		}

		if err := nodepoolImmutabilityCheck(&desired, current); err != nil {
			return nil, fmt.Errorf("immutability check for dynamic nodepools failed: %w", err)
		}
	}

	return nil, v.validate(ctx, newObj)
}

// ValidateDelete defines the logic when a kubernetes obj resource is deleted
func (v *InputManifestValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, v.validate(ctx, obj)
}

// validateInputManifest takes v1beta.InputManifest, validate its structure
// and returns an error when the validation will fail.
// It doesn't validate .spec.Providers field.
func validateInputManifest(im *v1beta.InputManifest) error {
	var rawManifest manifest.Manifest
	validateUniqueProviders := make(map[string]bool)
	// Fill providers only with names, to check if they are defined
	for _, p := range im.Spec.Providers {
		if _, exists := validateUniqueProviders[p.ProviderName]; exists {
			return fmt.Errorf("spec.providers.name has to be unique")
		}
		validateUniqueProviders[p.ProviderName] = true
		switch p.ProviderType {
		case v1beta.GCP:
			rawManifest.Providers.GCP = append(rawManifest.Providers.GCP, manifest.GCP{Name: p.ProviderName})
		case v1beta.AWS:
			rawManifest.Providers.AWS = append(rawManifest.Providers.AWS, manifest.AWS{Name: p.ProviderName})
		case v1beta.HETZNER:
			rawManifest.Providers.Hetzner = append(rawManifest.Providers.Hetzner, manifest.Hetzner{Name: p.ProviderName})
		case v1beta.OCI:
			rawManifest.Providers.OCI = append(rawManifest.Providers.OCI, manifest.OCI{Name: p.ProviderName})
		case v1beta.AZURE:
			rawManifest.Providers.Azure = append(rawManifest.Providers.Azure, manifest.Azure{Name: p.ProviderName})
		case v1beta.CLOUDFLARE:
			rawManifest.Providers.Cloudflare = append(rawManifest.Providers.Cloudflare, manifest.Cloudflare{Name: p.ProviderName})
		case v1beta.OPENSTACK:
			rawManifest.Providers.Openstack = append(rawManifest.Providers.Openstack, manifest.Openstack{Name: p.ProviderName})
		case v1beta.EXOSCALE:
			rawManifest.Providers.Exoscale = append(rawManifest.Providers.Exoscale, manifest.Exoscale{Name: p.ProviderName})
		case v1beta.CLOUDRIFT:
			rawManifest.Providers.CloudRift = append(rawManifest.Providers.CloudRift, manifest.CloudRift{Name: p.ProviderName})
		}
	}

	// Omit nodes as they contain secret reference to the private key
	for _, n := range im.Spec.NodePools.Static {
		rawManifest.NodePools.Static = append(rawManifest.NodePools.Static, manifest.StaticNodePool{Name: n.Name})
	}

	// Omit Envoy override as they're fetched during controller reconciliation.
	roles := make([]manifest.Role, 0, len(im.Spec.LoadBalancer.Roles))
	for _, r := range im.Spec.LoadBalancer.Roles {
		roles = append(roles, r.IntoManifestRole())
	}

	rawManifest.Name = im.GetNamespacedName()
	rawManifest.NodePools.Dynamic = im.Spec.NodePools.Dynamic
	rawManifest.Kubernetes = im.Spec.Kubernetes
	rawManifest.LoadBalancer = manifest.LoadBalancer{
		Roles:    roles,
		Clusters: im.Spec.LoadBalancer.Clusters,
	}

	// Run the validation of all field except the Provider Fields.
	// Providers will be validated separatly in the controller, after
	// it will gather all the credentials from the K8s Secret resources
	if err := rawManifest.Validate(); err != nil {
		return err
	}
	return nil
}

// getDynamicNodepoolsMap will read manifest from the given config and return map of provider names keyed by dynamic nodepool names
func getDynamicNodepoolsMap(m *v1beta.InputManifest) map[string]*manifest.DynamicNodePool {
	nmap := make(map[string]*manifest.DynamicNodePool)
	for _, np := range m.Spec.NodePools.Dynamic {
		nmap[np.Name] = &np
	}

	return nmap
}

func nodepoolImmutabilityCheck(desired, current *manifest.DynamicNodePool) error {
	if desired.ProviderSpec != current.ProviderSpec {
		return fmt.Errorf(
			"dynamic nodepools are immutable, changing the provider specification from %q to %q for %q is not allowed, only 'count' and 'autoscaling' fields are allowed to be modified, consider removing %q and creating a new one",
			safePrint(&current.ProviderSpec),
			safePrint(&desired.ProviderSpec),
			current.Name,
			current.Name,
		)
	}

	if desired.ServerType != current.ServerType {
		return fmt.Errorf(
			"dynamic nodepools are immutable, changing the server type, from %q to %q, for %q is not allowed, only 'count' and 'autoscaling' fields are allowed to be modified, consider removing %q and creating a new one",
			current.ServerType,
			desired.ServerType,
			current.Name,
			current.Name,
		)
	}

	if desired.Image != current.Image {
		return fmt.Errorf(
			"dynamic nodepools are immutable, changing the image from %q to %q for %q is not allowed, only 'count' and 'autoscaling' fields are allowed to be modified, consider removing %q and creating a new one",
			current.Image,
			desired.Image,
			current.Name,
			current.Name,
		)
	}

	storageDiskChanged := desired.StorageDiskSize == nil && current.StorageDiskSize != nil
	storageDiskChanged = storageDiskChanged || (desired.StorageDiskSize != nil && current.StorageDiskSize == nil)
	storageDiskChanged = storageDiskChanged || ((desired.StorageDiskSize != nil && current.StorageDiskSize != nil) && (*desired.StorageDiskSize != *current.StorageDiskSize))
	if storageDiskChanged {
		return fmt.Errorf(
			"dynamic nodepools are immutable, changing the storage disk size from %q to %q for %q is not allowed, only 'count' and 'autoscaling' fields are allowed to be modified, consider removing %q and creating a new one",
			safePrint(current.StorageDiskSize),
			safePrint(desired.StorageDiskSize),
			current.Name,
			current.Name,
		)
	}

	machineSpecChanged := desired.MachineSpec == nil && current.MachineSpec != nil
	machineSpecChanged = machineSpecChanged || (desired.MachineSpec != nil && current.MachineSpec == nil)
	machineSpecChanged = machineSpecChanged || ((desired.MachineSpec != nil && current.MachineSpec != nil) && (*desired.MachineSpec != *current.MachineSpec))
	if machineSpecChanged {
		return fmt.Errorf(
			"dynamic nodepools are immutable, changing the machine spec from %q to %q for %q is not allowed, only 'count' and 'autoscaling' fields are allowed to be modified, consider removing %q and creating a new one",
			safePrint(current.MachineSpec),
			safePrint(desired.MachineSpec),
			current.Name,
			current.Name,
		)
	}

	return nil
}

func safePrint[T any](p *T) string {
	if p == nil {
		return "<nil>"
	}

	return fmt.Sprintf("%+v", *p)
}
