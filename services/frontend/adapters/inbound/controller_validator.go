package inboundAdapters

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/berops/claudie/internal/manifest"
)

// secretValidator validates Secrets
type secretValidator struct{}

// validate will run on every incomming kubeapi validation request
func (v *secretValidator) validate(ctx context.Context, obj runtime.Object) error {
	log := crlog.FromContext(ctx)

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("expected a Secret but got a %T", obj)
	}

	log.Info("validating secret", "name", secret.Name, "namespace", secret.Namespace)
	if err := validateInputManifest(secret); err != nil {
		log.Error(err, "error validating secret", "name", secret.Name, "namespace", secret.Namespace)
		return err
	}

	log.V(1).Info("secret seems to be valid", "name", secret.Name, "namespace", secret.Namespace)
	return nil
}

func (v *secretValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj)
}

func (v *secretValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return v.validate(ctx, newObj)
}

func (v *secretValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return v.validate(ctx, obj)
}

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
