package utils

import (
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/server/domain/utils/secret"
)

type OutputType string

const (
	KubeconfigSecret OutputType = "kubeconfig"
	MetadataSecret   OutputType = "metadata"
)

// GetSecretMetadata returns metadata for secrets created in the management cluster as a Claudie output.
func GetSecretMetadata(ci *spec.ClusterInfo, projectName string, outputType OutputType) secret.Metadata {
	return secret.Metadata{
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
