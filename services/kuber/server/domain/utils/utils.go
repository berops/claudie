package utils

import (
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/server/domain/utils/secret"
)

type OutputType string

const (
	KubeconfigSecret OutputType = "kubeconfig"
	MetadataSecret   OutputType = "metadata"
)

// getSecretMetadata returns metadata for secrets created in the management cluster as a Claudie output.
func GetSecretMetadata(ci *spec.ClusterInfo, projectName string, outputType OutputType) secret.Metadata {
	cid := utils.GetClusterID(ci)
	return secret.Metadata{
		Name: fmt.Sprintf("%s-%s", cid, outputType),
		Labels: map[string]string{
			"claudie.io/project":        projectName,
			"claudie.io/cluster":        ci.Name,
			"claudie.io/cluster-id":     cid,
			"claudie.io/output":         string(outputType),
			"app.kubernetes.io/part-of": "claudie",
		},
	}
}
