package testingframework

import (
	"fmt"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	v1beta "github.com/berops/claudie/services/frontend/pkg/api/v1beta1"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	yaml "k8s.io/apimachinery/pkg/util/yaml"
)

// deleteInputManifest will delete an inputManifest from the cluster in the specified namespace
func deleteInputManifest(name string) error {
	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		kc.Stdout = comm.GetStdOut(name)
		kc.Stderr = comm.GetStdErr(name)
	}
	return kc.KubectlDeleteResource("inputmanifest", name, "-n", envs.Namespace)
}

// applyInputManifest function will create a inputManifest resource from the yaml file
// the name of the resource will be defined already in the file
func applyInputManifest(yamlFile []byte, pathToTestSet string) error {
	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		kc.Stdout = comm.GetStdOut(pathToTestSet)
		kc.Stderr = comm.GetStdErr(pathToTestSet)
	}
	return kc.KubectlApplyString(string(yamlFile), "-n", envs.Namespace)
}

// getInputManifestName will read the name of the manifest from the file and return it,
// so it can be used as an id to retrieve it from database in configChecker()
func getInputManifestName(yamlFile []byte) (string, error) {
	var manifest v1beta.InputManifest

	err := yaml.Unmarshal(yamlFile, &manifest)
	if err != nil {
		return "", fmt.Errorf("error while unmarshalling a manifest file: %w", err)
	}

	// Name is checked before apply, so ID needs to be combined manually (.metadata.namespace is not present before apply)
	if manifest.GetName() != "" {
		return envs.Namespace + "-" + manifest.GetName(), nil
	}
	return "", fmt.Errorf("manifest does not have a name defined, which could be used as DB id")
}
