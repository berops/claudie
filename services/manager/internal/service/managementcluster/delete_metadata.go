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
func DeleteClusterMetadata(manifestName string, clusters *spec.Clusters) error {
	namespace := envs.Namespace
	if namespace == "" {
		return nil
	}

	kc := kubectl.Kubectl{
		MaxKubectlRetries: kubectl.NoRetries,
		Stdout:            comm.GetStdOut(clusters.K8S.ClusterInfo.Id()),
		Stderr:            comm.GetStdErr(clusters.K8S.ClusterInfo.Id()),
	}

	return kc.KubectlDeleteResource("secret", fmt.Sprintf("%s-%s-metadata", manifestName, clusters.K8S.ClusterInfo.Name), "-n", namespace)
}
