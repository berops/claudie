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
		case v1beta.GENESIS_CLOUD:
			rawManifest.Providers.GenesisCloud = append(rawManifest.Providers.GenesisCloud, manifest.GenesisCloud{Name: p.ProviderName})
		case v1beta.OCI:
			rawManifest.Providers.OCI = append(rawManifest.Providers.OCI, manifest.OCI{Name: p.ProviderName})
		case v1beta.AZURE:
			rawManifest.Providers.Azure = append(rawManifest.Providers.Azure, manifest.Azure{Name: p.ProviderName})
		case v1beta.CLOUDFLARE:
			rawManifest.Providers.Cloudflare = append(rawManifest.Providers.Cloudflare, manifest.Cloudflare{Name: p.ProviderName})
		case v1beta.HETZNER_DNS:
			rawManifest.Providers.HetznerDNS = append(rawManifest.Providers.HetznerDNS, manifest.HetznerDNS{Name: p.ProviderName})
		case v1beta.OPENSTACK:
			rawManifest.Providers.Openstack = append(rawManifest.Providers.Openstack, manifest.Openstack{Name: p.ProviderName})
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
