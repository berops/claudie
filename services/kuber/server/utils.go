package main

import (
	"fmt"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/secret"
)

type outputType string

const (
	kubeconfigSecret outputType = "kubeconfig"
	metadataSecret   outputType = "metadata"
)

// getSecretMetadata returns metadata for secrets created in the management cluster as a Claudie output.
func getSecretMetadata(ci *pb.ClusterInfo, projectName string, outputType outputType) secret.Metadata {
	cid := utils.GetClusterID(ci)
	return secret.Metadata{
		Name: fmt.Sprintf("%s-%s", cid, outputType),
		Labels: map[string]string{
			"claudie.io/project":    projectName,
			"claudie.io/cluster":    ci.Name,
			"claudie.io/cluster-id": cid,
			"claudie.io/output":     string(outputType),
		},
	}
}
