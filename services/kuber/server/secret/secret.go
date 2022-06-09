package secret

import (
	"fmt"
	"os"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/kuber/server/kubectl"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Secret struct {
	Cluster      *pb.ClusterInfo
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
	Name   string        `yaml:"name"`
	Labels []interface{} `yaml:"labels"`
}

type Data struct {
	SecretData string
}

type Label struct {
	Label string `yaml:"claudie.io/input-manifest"`
}

const (
	secretYamlDir  string      = "services/kuber/server/secret/manifest"
	filePermission os.FileMode = 0644
)

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

func (s *Secret) Create() error {
	kubectl := kubectl.Kubectl{Kubeconfig: ""} // setting empty string for kubeconfig will create secret on same cluster where claudie is running
	namespace := os.Getenv("NAMESPACE")

	path, err := s.SaveSecretManifest()
	if err != nil {
		return err
	}
	err = kubectl.KubectlApply(path, namespace)
	if err != nil {
		return err
	}

	// cleanup
	if err = os.Remove(path); err != nil {
		return fmt.Errorf("Error while delete the manifest file")
	}
	return nil
}

func (s *Secret) SaveSecretManifest() (string, error) {
	secretYaml, err := yaml.Marshal(s.YamlManifest)
	if err != nil {
		log.Err(err).Msg("Failed to marshal secret manifest yaml")
		return "", err
	}
	// default file name
	var filename = "secret.yaml"
	if s.Cluster != nil {
		filename = fmt.Sprintf("%s-%s", s.Cluster.Name, s.Cluster.Hash)
	}
	path := fmt.Sprintf("%s/%s", secretYamlDir, filename)
	err = os.WriteFile(path, secretYaml, filePermission)
	if err != nil {
		log.Error().Msgf("Error while saving secret manifest file")
		return "", err
	}
	return path, nil
}
