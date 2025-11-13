package managementcluster

import (
	"fmt"
	"os"
	"path/filepath"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type OutputType string

const (
	KubeconfigSecret OutputType = "kubeconfig"
	MetadataSecret   OutputType = "metadata"
)

// SecretMetadata returns metadata for secrets created in the management cluster.
func SecretMetadata(ci *spec.ClusterInfoV2, projectName string, outputType OutputType) Metadata {
	return Metadata{
		Name: fmt.Sprintf("%s-%s", ci.Id(), outputType),
		Labels: map[string]string{
			"claudie.io/project":        projectName,
			"claudie.io/cluster":        ci.Name,
			"claudie.io/cluster-id":     ci.Id(),
			"claudie.io/output":         string(outputType),
			"app.kubernetes.io/part-of": "claudie",
		},
	}
}

// Secret holds information necessary to create a secret
type Secret struct {
	// Directory - directory where secret will be created
	Directory string

	// YamlManifest - secret specification
	YamlManifest SecretYaml
}

type SecretYaml struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   Metadata          `yaml:"metadata"`
	SecretType string            `yaml:"type"`
	Data       map[string]string `yaml:"data"`
}

type Metadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

const (
	filePermission os.FileMode = 0644
	filename                   = "secret.yaml"
)

// New create a k8s Secret manifest object from the specified manifest.
func NewSecret(directory string, secretYaml SecretYaml) Secret {
	return Secret{
		Directory:    directory,
		YamlManifest: secretYaml,
	}
}

// NewYaml created a template with pre-defined defaults and optional metadata & data fields.
func NewSecretYaml(md Metadata, data map[string]string) SecretYaml {
	return SecretYaml{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata:   md,
		SecretType: "Opaque",
		Data:       data,
	}
}

// Apply creates a secret manifest and applies it in the cluster (specified by given kubeconfig)
// in the specified namespace, if the kubeconfig is left empty, it uses default kubeconfig.
func (s *Secret) Apply(namespace string) error {
	kubectl := kubectl.Kubectl{
		// setting empty string for kubeconfig will create secret on same cluster where claudie is running
		Kubeconfig:        "",
		MaxKubectlRetries: -1,
	}
	kubectl.Stdout = comm.GetStdOut(s.YamlManifest.Metadata.Name)
	kubectl.Stderr = comm.GetStdErr(s.YamlManifest.Metadata.Name)

	defer func() {
		if err := os.RemoveAll(s.Directory); err != nil {
			log.Err(err).Msgf("failed to clean up dir %s", s.Directory)
		}
	}()

	if err := fileutils.CreateDirectory(s.Directory); err != nil {
		return fmt.Errorf("error while creating directory %s : %w", s.Directory, err)
	}

	secretYaml, err := yaml.Marshal(&s.YamlManifest)
	if err != nil {
		return fmt.Errorf("failed to marshal secret manifest yaml: %w", err)
	}

	path := filepath.Join(s.Directory, fmt.Sprintf("%v_%v", s.YamlManifest.Metadata.Name, filename))
	if err = os.WriteFile(path, secretYaml, filePermission); err != nil {
		return fmt.Errorf("error while saving secret manifest file %s : %w", path, err)
	}

	if err := kubectl.KubectlApply(path, "-n", namespace); err != nil {
		return fmt.Errorf("error while applying secret.yaml for %s : %w", s.YamlManifest.Metadata.Name, err)
	}

	return nil
}
