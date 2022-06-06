package testingframework

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Berops/platform/services/scheduler/manifest"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

type Secret struct {
	ApiVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	SecretType string   `yaml:"type"`
	Data       Data     `yaml:"data"`
}

type Metadata struct {
	Name   string `yaml:"name"`
	Labels Label  `yaml:"labels"`
}

type Data struct {
	Manifest string `yaml:"manifest"`
}

type Label struct {
	Label string `yaml:"claudie.io/input-manifest"`
}

var (
	secret = Secret{ApiVersion: "v1", Kind: "Secret", Metadata: Metadata{Labels: Label{}}, SecretType: "Opaque", Data: Data{}}
)

const (
	secretLabel = "claudie.io/input-manifest"
	secretFile  = "secret.yaml"
)

// createSecret function will create a secret.yaml file in test set directory, with a specified manifest in data encoded as base64 string
func createSecret(manifest []byte, secretName, testSetPath string) error {
	newSecret := secret
	newSecret.Data.Manifest = base64.StdEncoding.EncodeToString(manifest)
	newSecret.Metadata.Name = secretName
	newSecret.Metadata.Labels.Label = secretName

	secretYaml, err := yaml.Marshal(&newSecret)
	if err != nil {
		log.Error().Msgf("Error while marshalling the manifest %s : %v", secretName, err)
		return err
	}
	fmt.Println(string(secretYaml))

	err = os.WriteFile(fmt.Sprintf("%s/%s", testSetPath, secretFile), secretYaml, 0644)
	if err != nil {
		log.Error().Msgf("Error while creating secret.yaml for %s : %v", secretName, err)
		return err
	}
	return nil
}

// editSecret rewrites the old manifest with the new one as a base64 string
func editSecret(manifest []byte, pathToSecret string) error {
	var secretStruct Secret
	oldSecretYaml, err := ioutil.ReadFile(pathToSecret)
	if err != nil {
		log.Error().Msgf("Error while reading the secret.yaml in %s : %v", pathToSecret, err)
		return err
	}
	err = yaml.Unmarshal(oldSecretYaml, &secretStruct)
	if err != nil {
		log.Error().Msgf("Error while unmarshalling %s : %v", pathToSecret, err)
		return err
	}
	// replace the old manifest with new one
	secretStruct.Data.Manifest = base64.StdEncoding.EncodeToString(manifest)
	// create new yaml string
	secretYaml, err := yaml.Marshal(&secretStruct)
	if err != nil {
		log.Error().Msgf("Error while marshalling the manifest %s : %v", secretStruct.Metadata.Name, err)
		return err
	}
	// write the new yaml string to same file
	err = os.WriteFile(pathToSecret, secretYaml, 0644)
	if err != nil {
		log.Error().Msgf("Error while rewriting secret.yaml for %s : %v", secretStruct.Metadata.Name, err)
		return err
	}
	return nil
}

// deleteSecret will delete a secret in the cluster in the specified namespace
func deleteSecret(setName, namespace string) error {
	command := fmt.Sprintf("%s %s %s %s", "kubectl delete secret", setName, "-n", namespace)
	cmd := exec.Command("/bin/bash", "-c", command)
	return cmd.Run()
}

// manageSecret will create/edit a secret in the cluster and namespace the testing-framework is deployed
func manageSecret(setName, testSetPath, manifestPath, namespace string) error {
	manifest, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	// check if secret present
	command := fmt.Sprintf("%s %s -n %s", "kubectl get secret", setName, namespace)
	cmd := exec.Command("/bin/bash", "-c", command)
	err = cmd.Run()
	if err != nil {
		// error -> secret not found, create one
		err := createSecret(manifest, setName, testSetPath)
		if err != nil {
			return err
		}
		secretPath := filepath.Join(testSetPath, secretFile)
		err = applySecret(secretPath, namespace)
		if err != nil {
			return err
		}
	} else {
		pathToSecret := filepath.Join(testSetPath, secretFile)
		err := editSecret(manifest, pathToSecret)
		if err != nil {
			return err
		}
		secretPath := filepath.Join(testSetPath, secretFile)
		err = applySecret(secretPath, namespace)
		if err != nil {
			return err
		}
	}
	return nil
}

// applySecret will apply secret.yaml to the specified namespace
func applySecret(path, namespace string) error {
	command := fmt.Sprintf("kubectl apply -f %s -n %s", path, namespace)
	log.Info().Msgf(command)
	cmd := exec.Command("/bin/bash", "-c", command)
	err := cmd.Run()
	if err != nil {
		log.Fatal().Msgf("Error while applying a secret.yaml for %s : %v", path, err)
		return err
	}
	return nil
}

// getManifestIf will read the name of the manifest from the file and return it,
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
