package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/manifest"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v3"
)

func (u *Usecases) CreateConfig(ctx context.Context, inputManifest *manifest.Manifest, resourceName string, resourceNamespace string) error {
	inputManifestMarshalled, err := yaml.Marshal(inputManifest)
	if err != nil {
		log.Err(err).Msgf("Failed to marshal manifest %s. Skipping...", inputManifest.Name)
		return err
	}

	// Define config
	req := &managerclient.UpsertManifestRequest{
		Name:     inputManifest.Name,
		Manifest: &managerclient.Manifest{Raw: string(inputManifestMarshalled)},
		K8sCtx:   &managerclient.KubernetesContext{Name: resourceName, Namespace: resourceNamespace},
	}

	err = managerclient.Retry(log.Logger, fmt.Sprintf("UpsertManifest %q", req.Name), func() error {
		return u.Manager.UpsertManifest(ctx, req)
	})
	if err != nil {
		log.Err(err).Msgf("Failed to save config %v due to error. Skipping...", inputManifest.Name)
		return err
	}
	log.Info().Msgf("Created config for input manifest %s", inputManifest.Name)
	return nil
}

func (u *Usecases) DeleteConfig(ctx context.Context, name string) error {
	err := managerclient.Retry(log.Logger, "MarkForDeletion", func() error {
		return u.Manager.MarkForDeletion(ctx, &managerclient.MarkForDeletionRequest{Name: name})
	})
	if err != nil {
		log.Err(err).Msgf("Failed to trigger deletion for config %v due to error. Skipping...", name)
		return err
	}

	log.Info().Msgf("Config %s was successfully marked for deletion", name)
	return nil
}
