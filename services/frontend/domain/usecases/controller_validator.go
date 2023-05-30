package usecases

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/berops/claudie/internal/manifest"
)

// secretValidator validates Secrets containing the input-manifest
type SecretValidator struct{}

// validate takes the context and a kubernetes object as a parameter.
// It will extract the secret data out of the received obj and run manifest validation against it
func (v *SecretValidator) validate(ctx context.Context, obj runtime.Object) error {
	log := crlog.FromContext(ctx)

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("expected a Secret but got a %T", obj)
	}

	log.Info("validating secret", "name", secret.Name)
	if err := validateInputManifest(secret); err != nil {
		log.Error(err, "error validating secret", "name", secret.Name)
		return err
	}

	log.V(1).Info("secret seems to be valid", "name", secret.Name)
	return nil
}

// ValidateCreate defines the logic when a kubernetes obj resource is created
func (v *SecretValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj)
}

// ValidateUpdate defines the logic when a kubernetes obj resource is updated
func (v *SecretValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return v.validate(ctx, newObj)
}

// ValidateDelete defines the logic when a kubernetes obj resource is deleted
func (v *SecretValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj)
}

// validateInputManifest takes a corev1.Secret, and return an error if one of the secret data will not pass the input-manifest validation
func validateInputManifest(secret *corev1.Secret) error {
	unmarshalledManifest := &manifest.Manifest{}

	// Iterate over the files in secret, try to unmarshall them and validate
	for name := range secret.Data {
		// Unmarshal input manifest from secret to manifest.Manifest type
		if err := yaml.Unmarshal(secret.Data[name], &unmarshalledManifest); err != nil {
			return fmt.Errorf("cannot unmarshal field %s from secret %s", name, secret.Name)
		}

		// Validate input manifest
		if err := unmarshalledManifest.Validate(); err != nil {
			return err
		}
	}

	return nil
}
