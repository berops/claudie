package usecases

import (
	"fmt"

	"github.com/berops/claudie/internal/cluster"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v3"
)

// If the workflow happens correctly, the desired state for the K8s cluster
// (represented by request.DesiredCluster) has already been reached in the kube-eleven microservice.
// Inside the K8s cluster, in the kube-public namespace there is a configmap named 'cluster-info'
// which holds the kubeconfig for this cluster.
// Currently, that kubeconfig represents the older state of this cluster.
// PatchClusterInfoConfigMap updates that kubeconfig so that it represents the already reached
// desired state of the cluster.
func (u *Usecases) PatchClusterInfoConfigMap(request *pb.PatchClusterInfoConfigMapRequest) (*pb.PatchClusterInfoConfigMapResponse, error) {
	logger := loggerutils.WithClusterName(cluster.Id(request.DesiredCluster.ClusterInfo))
	logger.Info().Msgf("Patching cluster info ConfigMap")

	k := kubectl.Kubectl{
		Kubeconfig: request.DesiredCluster.Kubeconfig,
	}

	configMap, err := k.KubectlGet("cm cluster-info", "-ojson", "-n kube-public")
	if err != nil {
		return nil, err
	}

	if configMap == nil {
		logger.Warn().Msgf("Cluster-info config map was not found in the cluster")
		return &pb.PatchClusterInfoConfigMapResponse{}, nil
	}

	configMapKubeconfig := gjson.Get(string(configMap), "data.kubeconfig")

	var rawKubeconfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(request.DesiredCluster.Kubeconfig), &rawKubeconfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubeconfig, malformed yaml : %w", err)
	}

	var rawConfigMapKubeconfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(configMapKubeconfig.String()), &rawConfigMapKubeconfig); err != nil {
		return nil, fmt.Errorf("failed to update cluster info config map, malformed yaml : %w", err)
	}

	// Kubeadm uses this config when joining nodes thus we need to update it with the new endpoint
	// https://kubernetes.io/docs/reference/setup-tools/kubeadm/implementation-details/#shared-token-discovery

	// only update the certificate-authority-data and server
	newClusters := rawKubeconfig["clusters"].([]interface{})
	if len(newClusters) == 0 {
		return nil, fmt.Errorf("desired state kubeconfig has no clusters")
	}
	newClusterInfo := newClusters[0].(map[string]interface{})["cluster"].(map[string]interface{})

	configMapClusters := rawConfigMapKubeconfig["clusters"].([]interface{})
	if len(configMapClusters) == 0 {
		return nil, fmt.Errorf("config-map kubeconfig has no clusters")
	}
	oldClusterInfo := configMapClusters[0].(map[string]interface{})["cluster"].(map[string]interface{})

	oldClusterInfo["server"] = newClusterInfo["server"]
	oldClusterInfo["certificate-authority-data"] = newClusterInfo["certificate-authority-data"]

	b, err := yaml.Marshal(rawConfigMapKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patched config map : %w", err)
	}

	patchedConfigMap, err := sjson.Set(string(configMap), "data.kubeconfig", b)
	if err != nil {
		return nil, fmt.Errorf("failed to update config map with new kubeconfig : %w", err)
	}

	if err = k.KubectlApplyString(patchedConfigMap, "-n kube-public"); err != nil {
		return nil, fmt.Errorf("failed to patch config map: %w", err)
	}

	logger.Info().Msgf("Cluster-info Config Map patched successfully")
	return &pb.PatchClusterInfoConfigMapResponse{}, nil
}
