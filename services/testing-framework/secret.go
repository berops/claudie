package testingframework

import (
	"encoding/base64"
	"fmt"

	"github.com/Berops/platform/services/kuber/server/kubectl"
	"github.com/Berops/platform/services/kuber/server/secret"
	"github.com/Berops/platform/services/scheduler/manifest"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

type Label struct {
	Label string `yaml:"claudie.io/input-manifest"`
}

// deleteSecret will delete a secret in the cluster in the specified namespace
func deleteSecret(setName, namespace string) error {
	kc := kubectl.Kubectl{}
	return kc.KubectlDeleteResource("secret", setName, namespace)
}

// manageSecret function will create a secret.yaml file in test set directory, with a specified manifest in data encoded as base64 string
func manageSecret(manifest []byte, pathToTestSet, secretName, namespace string) error {
	s := secret.New()
	s.Directory = pathToTestSet
	s.YamlManifest.Data.SecretData = base64.StdEncoding.EncodeToString(manifest)
	s.YamlManifest.Metadata.Name = secretName
	s.YamlManifest.Metadata.Labels = Label{Label: secretName}
	// apply secret
	err := s.Apply(namespace, "")
	if err != nil {
		log.Error().Msgf("Error while creating secret.yaml for %s : %v", secretName, err)
		return err
	}
	return nil
}

// getManifestId will read the name of the manifest from the file and return it,
// so it can be used as an id to retrieve it from database in configChecker()
func getManifestId(yamlFile []byte) (string, error) {
	var manifest manifest.Manifest
	err := yaml.Unmarshal(yamlFile, &manifest)
	if err != nil {
		log.Error().Msgf("Error while unmarshalling a manifest file: %v", err)
		return "", err
	}

	if manifest.Name != "" {
		return manifest.Name, nil
	}
	return "", fmt.Errorf("manifest does not have a name defined, which could be used as DB id")
}
