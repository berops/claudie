package secret

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Berops/platform/services/kuber/server/kubectl"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// struct Secret holds information necessary to create a secret
// Directory - directory where secret will be created
// YamlManifest - secret specification
type Secret struct {
	Directory    string
	YamlManifest SecretYaml
}

type SecretYaml struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	SecretType string   `yaml:"type"`
	Data       Data     `yaml:"data"`
}

type Metadata struct {
	Name   string      `yaml:"name"`
	Labels interface{} `yaml:"labels"`
}

type Data struct {
	SecretData string
}

const (
	filePermission os.FileMode = 0644
	filename                   = "secret.yaml"
)

// returns new Secret object with default values
func New() Secret {
	return Secret{
		YamlManifest: SecretYaml{
			APIVersion: "v1",
			Kind:       "Secret",
			Metadata:   Metadata{},
			SecretType: "Opaque",
			Data:       Data{SecretData: ""},
		},
	}
}

// Creates a secret manifests and applies it in the cluster (specified by given kubeconfig) in the specified namespace
// if the kubeconfig is left empty, it uses default kubeconfig
func (s *Secret) Apply(namespace, kubeconfig string) error {
	// setting empty string for kubeconfig will create secret on same cluster where claudie is running
	kubectl := kubectl.Kubectl{Kubeconfig: kubeconfig}
	path := filepath.Join(s.Directory, filename)

	err := s.saveSecretManifest(path)
	if err != nil {
		return fmt.Errorf("error while saving secret.yaml for %s : %v", s.YamlManifest.Metadata.Name, err)
	}
	err = kubectl.KubectlApply(path, namespace)
	if err != nil {
		return fmt.Errorf("error while applying secret.yaml for %s : %v", s.YamlManifest.Metadata.Name, err)
	}

	// cleanup
	if err = os.RemoveAll(s.Directory); err != nil {
		return fmt.Errorf("error while delete the secret.yaml for %s : %v", s.YamlManifest.Metadata.Name, err)
	}
	return nil
}

//saves secret into the file system
func (s *Secret) saveSecretManifest(path string) error {
	secretYaml, err := yaml.Marshal(&s.YamlManifest)
	if err != nil {
		log.Err(err).Msg("Failed to marshal secret manifest yaml")
		return err
	}

	err = os.WriteFile(path, secretYaml, filePermission)
	if err != nil {
		log.Error().Msgf("Error while saving secret manifest file")
		return err
	}
	return nil
}
