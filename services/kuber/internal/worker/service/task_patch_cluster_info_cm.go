package service

import (
	"errors"
	"fmt"

	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"gopkg.in/yaml.v3"
)

// Inside the K8s cluster, in the kube-public namespace there is a configmap named 'cluster-info'
// which holds the kubeconfig for this cluster. On changes to the API server, that kubeconfig
// epresents the older state of this cluster. This function updates that kubeconfig so that it
// represents the state present in the received task.
func PatchClusterInfoCM(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Patching cluster-info config map")

	action, ok := tracker.Task.Do.(*spec.TaskV2_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task %T while wanting to update cluster-info config map,"+
				" assuming the task was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	kubeconfig := action.Update.State.K8S.Kubeconfig
	patchClusterInfoCM(logger, kubeconfig, tracker)
}

func patchClusterInfoCM(logger zerolog.Logger, kubeconfig string, tracker Tracker) {
	k := kubectl.Kubectl{
		Kubeconfig: kubeconfig,
	}

	configMap, err := k.KubectlGet("cm cluster-info", "-ojson", "-n kube-public")
	if err != nil {
		logger.Err(err).Msg("Failed to fetch cluster-info config map")
		tracker.Diagnostics.Push(err)
		return
	}

	if configMap == nil {
		logger.Warn().Msgf("Cluster-info config map was not found in the cluster, ignoring patch operation")
		return
	}

	configMapKubeconfig := gjson.Get(string(configMap), "data.kubeconfig")

	var rawKubeconfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(kubeconfig), &rawKubeconfig); err != nil {
		logger.Err(err).Msg("Failed to unmarshal kubeconfig fron the State of the Update task")
		tracker.Diagnostics.Push(err)
		return
	}

	var rawConfigMapKubeconfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(configMapKubeconfig.String()), &rawConfigMapKubeconfig); err != nil {
		logger.Err(err).Msg("Failed to update cluster-info config map, malformed yaml")
		tracker.Diagnostics.Push(err)
		return
	}

	// Kubeadm uses this config when joining nodes thus we need to update it with the new endpoint
	// https://kubernetes.io/docs/reference/setup-tools/kubeadm/implementation-details/#shared-token-discovery

	// only update the certificate-authority-data and server
	newClusters := rawKubeconfig["clusters"].([]interface{})
	if len(newClusters) == 0 {
		err := errors.New("kubeconfig provided with the kubernetes cluster in the task has no clusters")
		logger.Err(err).Msg("Unexpected kubeconfig")
		tracker.Diagnostics.Push(err)
		return
	}
	newClusterInfo := newClusters[0].(map[string]interface{})["cluster"].(map[string]interface{})

	configMapClusters := rawConfigMapKubeconfig["clusters"].([]interface{})
	if len(configMapClusters) == 0 {
		err := errors.New("config-map kubeconfig has no clusters")
		logger.Err(err).Msg("Unexpected kubeconfig")
		tracker.Diagnostics.Push(err)
		return
	}
	oldClusterInfo := configMapClusters[0].(map[string]interface{})["cluster"].(map[string]interface{})

	oldClusterInfo["server"] = newClusterInfo["server"]
	oldClusterInfo["certificate-authority-data"] = newClusterInfo["certificate-authority-data"]

	b, err := yaml.Marshal(rawConfigMapKubeconfig)
	if err != nil {
		err := fmt.Errorf("failed to marshal patched config map : %w", err)
		logger.Err(err).Msg("Marshalling changes failed")
		tracker.Diagnostics.Push(err)
		return
	}

	patchedConfigMap, err := sjson.Set(string(configMap), "data.kubeconfig", b)
	if err != nil {
		err := fmt.Errorf("failed to update config map with new kubeconfig : %w", err)
		logger.Err(err).Msg("Patching config map failed")
		tracker.Diagnostics.Push(err)
		return
	}

	if err = k.KubectlApplyString(patchedConfigMap, "-n kube-public"); err != nil {
		logger.Err(err).Msg("Failed to apply newly patched cluster-info config map")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msgf("Cluster-info Config Map patched successfully")
}
