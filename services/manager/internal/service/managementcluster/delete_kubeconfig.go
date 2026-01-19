package managementcluster

import (
	"fmt"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
)

// DeleteKubeconfig deletes the K8s secret (in the management cluster) containing kubeconfig
// for the given K8s cluster.
func DeleteKubeconfig(clusters *spec.Clusters) error {
	namespace := envs.Namespace
	clusterID := clusters.K8S.ClusterInfo.Id()
	if namespace == "" {
		return nil
	}

	kc := kubectl.Kubectl{
		MaxKubectlRetries: -1,
	}

	kc.Stdout = comm.GetStdOut(clusterID)
	kc.Stderr = comm.GetStdErr(clusterID)

	return kc.KubectlDeleteResource("secret", fmt.Sprintf("%s-kubeconfig", clusterID), "-n", namespace)
}
