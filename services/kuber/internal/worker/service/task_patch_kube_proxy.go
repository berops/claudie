package service

import (
	"errors"
	"fmt"

	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"

	"gopkg.in/yaml.v3"
)

func PatchKubeProxy(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Patching kube proxy")

	action, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task %T while wanting to patch kube-proxy config map,"+
				" assuming the task was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	kubeconfig := action.Update.State.K8S.Kubeconfig
	patchKubeProxy(logger, kubeconfig, tracker)
}

func patchKubeProxy(logger zerolog.Logger, kubeconfig string, tracker Tracker) {
	k := kubectl.Kubectl{
		Kubeconfig: kubeconfig,
	}

	configMap, err := k.KubectlGet("cm kube-proxy", "-oyaml", "-n kube-system")
	if err != nil {
		logger.Err(err).Msg("Failed to fetch kube-proxy")
		tracker.Diagnostics.Push(err)
		return
	}

	if configMap == nil {
		logger.Warn().Msg("Kube Proxy config map empty, skip patching")
		return
	}

	var desiredKubeconfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(kubeconfig), &desiredKubeconfig); err != nil {
		err := fmt.Errorf("failed to unmarshal kubeconfig, malformed yaml : %w", err)
		logger.Err(err).Msg("Unmarshall failed")
		tracker.Diagnostics.Push(err)
		return
	}

	var rawConfigMap map[string]interface{}
	if err := yaml.Unmarshal(configMap, &rawConfigMap); err != nil {
		err := fmt.Errorf("failed to update cluster info config map, malformed yaml : %w", err)
		logger.Err(err).Msg("Failed to unmarshall")
		tracker.Diagnostics.Push(err)
		return
	}

	// get the new api server address
	desiredCluster, err := extractClusterFromKubeconfig(desiredKubeconfig)
	if err != nil {
		err := fmt.Errorf("failed to extract cluster data from kubeconfing: %w", err)
		logger.Err(err).Msg("Unexpected kubeconfig structure")
		tracker.Diagnostics.Push(err)
		return
	}

	if err := updateKubeconfigServerEndpoint(rawConfigMap, desiredCluster["server"]); err != nil {
		err := fmt.Errorf("failed to patch kube-proxy kubeconfig: %w", err)
		logger.Err(err).Msg("Unexpected config map structure")
		tracker.Diagnostics.Push(err)
		return
	}

	b, err := yaml.Marshal(rawConfigMap)
	if err != nil {
		err := fmt.Errorf("failed to marshal patched config map : %w", err)
		logger.Err(err).Msg("Failed to marshall")
		tracker.Diagnostics.Push(err)
		return
	}

	if err = k.KubectlApplyString(string(b), "-n kube-system"); err != nil {
		err := fmt.Errorf("failed to patch config map: %w", err)
		logger.Err(err).Msg("Failed to apply patched kube proxy config map")
		tracker.Diagnostics.Push(err)
		return
	}

	// Delete old kube-proxy pods to use updated config-map
	if err := k.KubectlDeleteResource("pods", "-l k8s-app=kube-proxy", "-n kube-system"); err != nil {
		err := fmt.Errorf("failed to restart kube-proxy pods: %w", err)
		logger.Err(err).Msg("Failed to delete old kube-proxy pods to force new changes")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msgf("Kube-proxy Config Map patched successfully")
}

func extractClusterFromKubeconfig(root map[string]any) (map[string]any, error) {
	clustersSlice, ok := root["clusters"].([]any)
	if !ok {
		return nil, errors.New("expected 'clusters' to be an []any")
	}
	if len(clustersSlice) == 0 {
		return nil, errors.New("no clusters to patch")
	}

	cluster, ok := clustersSlice[0].(map[string]any)
	if !ok {
		return nil, errors.New("expected clusters to be of type map[string]any")
	}

	clusterData, ok := cluster["cluster"].(map[string]any)
	if !ok {
		return nil, errors.New("expeted clusters of the kubeconfig to be of type map[string]any")
	}

	return clusterData, nil
}

func updateKubeconfigServerEndpoint(root map[string]any, val any) error {
	data, ok := root["data"].(map[string]any)
	if !ok {
		return errors.New("expected 'data' field to be a map[string]any")
	}

	kubeConf, ok := data["kubeconfig.conf"].(string)
	if !ok {
		return errors.New("expected 'kubeconfig.conf' to be a string")
	}

	var kubeConfMap map[string]any
	if err := yaml.Unmarshal([]byte(kubeConf), &kubeConfMap); err != nil {
		return err
	}

	cluster, err := extractClusterFromKubeconfig(kubeConfMap)
	if err != nil {
		return err
	}

	cluster["server"] = val

	b, err := yaml.Marshal(kubeConfMap)
	if err != nil {
		return err
	}

	data["kubeconfig.conf"] = string(b)
	return nil
}
