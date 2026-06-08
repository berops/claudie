package managementcluster

import (
	"encoding/base64"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/proto/pb/spec"
)

func StoreKubeconfig(manifestName string, clusters *spec.Clusters) error {
	if envs.Namespace == "" {
		return nil
	}

	var (
		clusterID      = clusters.K8S.ClusterInfo.Id()
		clusterDir     = filepath.Join(outputDir, clusterID)
		encodedData    = base64.StdEncoding.EncodeToString([]byte(clusters.K8S.Kubeconfig))
		secretData     = map[string]string{"kubeconfig": encodedData}
		secretMetadata = SecretMetadata(clusters.K8S.ClusterInfo, manifestName, KubeconfigSecret)
		secretYaml     = NewSecretYaml(secretMetadata, secretData)
		secret         = NewSecret(clusterDir, secretYaml)
	)

	if err := secret.Apply(envs.Namespace); err != nil {
		return fmt.Errorf("error while creating the kubeconfig secret: %w", err)
	}

	return nil
}
