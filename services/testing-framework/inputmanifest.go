package testingframework

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/yaml"

	v1beta "github.com/berops/claudie/internal/api/crd/inputmanifest/v1beta1"
	"github.com/berops/claudie/internal/api/manifest"
	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/rs/zerolog/log"
)

// deleteInputManifest will delete an inputManifest from the cluster in the specified namespace
func deleteInputManifest(name string) error {
	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	kc.Stdout = comm.GetStdOut(name)
	kc.Stderr = comm.GetStdErr(name)

	return kc.KubectlDeleteResource("inputmanifest", name, "-n", envs.Namespace)
}

// applyInputManifest function will create a inputManifest resource from the yaml file
// the name of the resource will be defined already in the file
func applyInputManifest(yamlFile []byte, pathToTestSet string) error {
	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	kc.Stdout = comm.GetStdOut(pathToTestSet)
	kc.Stderr = comm.GetStdErr(pathToTestSet)

	return kc.KubectlApplyString(string(yamlFile), "-n", envs.Namespace)
}

// getInputManifestName will read the name of the manifest from the file and return it,
// so it can be used as an id to retrieve it from database in configChecker()
func getInputManifestName(yamlFile []byte) (string, error) {
	// Local testing
	if envs.Namespace == "" {
		var manifest manifest.Manifest
		if err := yaml.Unmarshal(yamlFile, &manifest); err != nil {
			return "", fmt.Errorf("error while unmarshalling a manifest file: %w", err)
		}
		log.Debug().Msgf("Returning name %s for", manifest.Name)
		return manifest.Name, nil
	}

	var manifest v1beta.InputManifest
	err := yaml.Unmarshal(yamlFile, &manifest)
	if err != nil {
		return "", fmt.Errorf("error while unmarshalling a manifest file: %w", err)
	}

	// Name is checked before apply, so ID needs to be combined manually (.metadata.namespace is not present before apply)
	if manifest.Name != "" {
		return envs.Namespace + "-" + manifest.GetName(), nil
	}
	return "", fmt.Errorf("manifest does not have a name defined, which could be used as DB id")
}
