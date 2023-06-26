package usecases

import (
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb"
)

// createConfig generates and saves config into the DB. Used for new configs and updated configs.
func (u *Usecases) CreateConfig(inputManifest *manifest.Manifest, resourceName string, resourceNamespace string) error {
	inputManifestMarshalled, err := yaml.Marshal(inputManifest)
	if err != nil {
		log.Err(err).Msgf("Failed to marshal manifest %s. Skipping...", inputManifest.Name)
		return err
	}

	// Define config
	config := &pb.Config{
		Name:              inputManifest.Name,
		ResourceName:      resourceName,
		ResourceNamespace: resourceNamespace,
		Manifest:          string(inputManifestMarshalled),
	}

	if err := u.ContextBox.SaveConfig(config); err != nil {
		log.Err(err).Msgf("Failed to save config %v due to error. Skipping...", inputManifest.Name)
		return err
	}
	log.Info().Msgf("Created config for input manifest %s", inputManifest.Name)
	return nil
}

// deleteConfig generates and triggers deletion of config into the DB.
func (u *Usecases) DeleteConfig(inputManifest *manifest.Manifest) error {
	if err := u.ContextBox.DeleteConfig(inputManifest.Name); err != nil {
		log.Err(err).Msgf("Failed to trigger deletion for config %v due to error. Skipping...", inputManifest.Name)
		return err
	}

	log.Info().Msgf("Config %s was successfully marked for deletion", inputManifest.Name)
	return nil
}
