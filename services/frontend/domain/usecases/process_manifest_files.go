package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb"
)

// ProcessManifestFiles processes the manifest files concurrently. If an error occurs while the file
// is being processed, it's skipped and the function continues with the next one until all files are
// processed. Nothing is done with those files for which errors occurred, they'll be skipped until either
// corrected or deleted.
func (u *Usecases) ProcessManifestFiles() error {

	select {
	case newManifest := <-u.CreateChannel:
		go u.createConfig(newManifest)
	case newManifest := <-u.DeleteChannel:
		go u.deleteConfig(newManifest)
	}
	return nil
}

func (u *Usecases) createConfig(rawManifest *RawManifest) {
	unmarshalledManifest := &manifest.Manifest{}
	if err := yaml.Unmarshal(rawManifest.Manifest, &unmarshalledManifest); err != nil {
		log.Err(err).Msgf("Failed to unmarshal manifest from YAML file %s form secret %s. Skipping...", rawManifest.FileName, rawManifest.SecretName)
		return
	}
	if err := unmarshalledManifest.Validate(); err != nil {
		log.Err(err).Msgf("Failed to validate manifest %s from secret %s. Skipping...", unmarshalledManifest.Name, rawManifest.SecretName)
	}
	config := &pb.Config{
		Name:             unmarshalledManifest.Name,
		ManifestFileName: fmt.Sprintf("secret_%s.file_%s", rawManifest.SecretName, rawManifest.FileName),
		Manifest:         string(rawManifest.Manifest),
	}

	err := u.ContextBox.SaveConfig(config)
	if err != nil {
		log.Err(err).Msgf("Failed to save config %v due to error. Skipping...", unmarshalledManifest.Name)
		return
	}
	log.Info().Msgf("Created config for input manifest %s", unmarshalledManifest.Name)
}

func (u *Usecases) deleteConfig(rawManifest *RawManifest) {
	unmarshalledManifest := &manifest.Manifest{}
	if err := yaml.Unmarshal(rawManifest.Manifest, &unmarshalledManifest); err != nil {
		log.Err(err).Msgf("Failed to unmarshal manifest from YAML file %s form secret %s. Skipping...", rawManifest.FileName, rawManifest.SecretName)
		return
	}
	err := u.ContextBox.DeleteConfig(unmarshalledManifest.Name)
	if err != nil {
		log.Err(err).Msgf("Failed to trigger deletion for config %v due to error. Skipping...", unmarshalledManifest.Name)
		return
	}
	log.Info().Msgf("Config %s was successfully marked for deletion", unmarshalledManifest.Name)
}
