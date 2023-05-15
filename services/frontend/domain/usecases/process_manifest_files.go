package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) ProcessManifestFiles() {
	for {
		select {
		case newManifest := <-u.SaveChannel:
			go u.createConfig(newManifest)
		case newManifest := <-u.DeleteChannel:
			go u.deleteConfig(newManifest)
		case <-u.Context.Done():
			// Close channels and return
			close(u.SaveChannel)
			close(u.DeleteChannel)
			return
		}
	}
}

func (u *Usecases) createConfig(rawManifest *RawManifest) {
	unmarshalledManifest := &manifest.Manifest{}
	// Unmarshal
	if err := yaml.Unmarshal(rawManifest.Manifest, &unmarshalledManifest); err != nil {
		log.Err(err).Msgf("Failed to unmarshal manifest from YAML file %s form secret %s. Skipping...", rawManifest.FileName, rawManifest.SecretName)
		return
	}

	// Validate
	if err := unmarshalledManifest.Validate(); err != nil {
		log.Err(err).Msgf("Failed to validate manifest %s from secret %s. Skipping...", unmarshalledManifest.Name, rawManifest.SecretName)
		return
	}
	// Define config
	config := &pb.Config{
		Name:             unmarshalledManifest.Name,
		ManifestFileName: fmt.Sprintf("secret_%s.file_%s", rawManifest.SecretName, rawManifest.FileName),
		Manifest:         string(rawManifest.Manifest),
	}

	if err := u.ContextBox.SaveConfig(config); err != nil {
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

	if err := u.ContextBox.DeleteConfig(unmarshalledManifest.Name); err != nil {
		log.Err(err).Msgf("Failed to trigger deletion for config %v due to error. Skipping...", unmarshalledManifest.Name)
		return
	}

	log.Info().Msgf("Config %s was successfully marked for deletion", unmarshalledManifest.Name)
}
