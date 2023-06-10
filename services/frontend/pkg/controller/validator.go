package controller

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/manifest"
	v1beta "github.com/berops/claudie/services/frontend/pkg/api/v1beta1"
	wbhk "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
)

// ImputManifestValidator validates InputManifest containing the input-manifest
type ImputManifestValdator struct {
	Logger logr.Logger
}

// NewWebhook returns a new validation webhook for InputManifest resource
func NewWebhook(port int, dir, path string, log logr.Logger) *wbhk.Server{
	hookServer := &wbhk.Server{
		Port:    port,
		CertDir: dir,
	}
	hookServer.Register(path, admission.WithCustomValidator(&v1beta.InputManifest{}, &ImputManifestValdator{log}))
	return hookServer
}

// validate takes the context and a kubernetes object as a parameter.
// It will extract the secret data out of the received obj and run manifest validation against it
func (v *ImputManifestValdator) validate(ctx context.Context, obj runtime.Object) error {
	log := v.Logger.WithName("InputManifest Validator")

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
func (v *ImputManifestValdator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj)
}

// ValidateUpdate defines the logic when a kubernetes obj resource is updated
func (v *ImputManifestValdator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return v.validate(ctx, newObj)
}

// ValidateDelete defines the logic when a kubernetes obj resource is deleted
func (v *ImputManifestValdator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj)
}

func validateInputManifest(im *v1beta.InputManifest) error {

	var rawManifest manifest.Manifest

	// Fill providers only with names, to check if there are defined
	for _, p := range im.Spec.Providers {
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
		case v1beta.HETZNER_DNS:
			rawManifest.Providers.HetznerDNS = append(rawManifest.Providers.HetznerDNS, manifest.HetznerDNS{Name: p.ProviderName})
		}
	}

	rawManifest.Name = im.GetNamespacedName()
	rawManifest.NodePools = im.Spec.NodePools
	rawManifest.Kubernetes = im.Spec.Kubernetes
	rawManifest.LoadBalancer = im.Spec.LoadBalancer

	// Run the validation of all field except the Provider Fields.
	// Providers will be validated separatly in the controller, after
	// it will gather all the credentials from the K8s Secret resources
	if err := rawManifest.Kubernetes.Validate(&rawManifest); err != nil {
		return err
	}
	if err := rawManifest.NodePools.Validate(&rawManifest); err != nil {
		return err
	}
	if err := rawManifest.LoadBalancer.Validate(&rawManifest); err != nil {
		return err
	}
	if err := manifest.CheckLengthOfFutureDomain(&rawManifest); err != nil {
		return err
	}

	return nil
}
