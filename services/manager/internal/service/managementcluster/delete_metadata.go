package managementcluster

import (
	"fmt"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
)

// DeleteClusterMetadata deletes the K8s secret (from the management cluster) containing cluster
// metadata for the given K8s cluster.
func DeleteClusterMetadata(clusters *spec.Clusters) error {
	namespace := envs.Namespace
	clusterID := clusters.K8S.ClusterInfo.Id()

	if namespace == "" {
		return nil
	}

	kc := kubectl.Kubectl{
		MaxKubectlRetries: -1,
		Stdout:            comm.GetStdOut(clusterID),
		Stderr:            comm.GetStdErr(clusterID),
	}

	return kc.KubectlDeleteResource("secret", fmt.Sprintf("%s-metadata", clusterID), "-n", namespace)
}
