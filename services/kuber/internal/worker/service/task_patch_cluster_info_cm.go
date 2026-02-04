package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

	action, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task %T while wanting to update cluster-info config map,"+
				" assuming the task was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	kubeconfig := action.Update.State.K8S.Kubeconfig
	clusterId := action.Update.State.K8S.ClusterInfo.Id()
	patchClusterInfoCM(logger, kubeconfig, clusterId, tracker)
}

func patchClusterInfoCM(logger zerolog.Logger, kubeconfig, clusterId string, tracker Tracker) {
	clusterDir := filepath.Join(OutputDir, fmt.Sprintf("%s-%s", clusterId, hash.Create(7)))
	if err := fileutils.CreateDirectory(clusterDir); err != nil {
		logger.Err(err).Msgf("Failed to create directory %s", clusterDir)
		tracker.Diagnostics.Push(err)
		return
	}

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Err(err).Msgf("error while deleting temp directory: %s", clusterDir)
		}
	}()

	file, err := os.CreateTemp(clusterDir, clusterId)
	if err != nil {
		err := fmt.Errorf("failed to create temporary file: %w", err)
		logger.Err(err).Msg("Failed to create temp file within temp directory")
		tracker.Diagnostics.Push(err)
		return
	}
	defer file.Close()

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

	var rawKubeconfig map[string]any
	if err := yaml.Unmarshal([]byte(kubeconfig), &rawKubeconfig); err != nil {
		logger.Err(err).Msg("Failed to unmarshal kubeconfig fron the State of the Update task")
		tracker.Diagnostics.Push(err)
		return
	}

	var rawConfigMapKubeconfig map[string]any
	if err := yaml.Unmarshal([]byte(configMapKubeconfig.String()), &rawConfigMapKubeconfig); err != nil {
		logger.Err(err).Msg("Failed to update cluster-info config map, malformed yaml")
		tracker.Diagnostics.Push(err)
		return
	}

	// Kubeadm uses this config when joining nodes thus we need to update it with the new endpoint
	// https://kubernetes.io/docs/reference/setup-tools/kubeadm/implementation-details/#shared-token-discovery

	// only update the certificate-authority-data and server
	newClusters := rawKubeconfig["clusters"].([]any)
	if len(newClusters) == 0 {
		err := errors.New("kubeconfig provided with the kubernetes cluster in the task has no clusters")
		logger.Err(err).Msg("Unexpected kubeconfig")
		tracker.Diagnostics.Push(err)
		return
	}
	newClusterInfo := newClusters[0].(map[string]any)["cluster"].(map[string]any)

	configMapClusters := rawConfigMapKubeconfig["clusters"].([]any)
	if len(configMapClusters) == 0 {
		err := errors.New("config-map kubeconfig has no clusters")
		logger.Err(err).Msg("Unexpected kubeconfig")
		tracker.Diagnostics.Push(err)
		return
	}
	oldClusterInfo := configMapClusters[0].(map[string]any)["cluster"].(map[string]any)

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

	n, err := io.Copy(file, strings.NewReader(patchedConfigMap))
	if err != nil {
		err := fmt.Errorf("failed to write contents to temporary file: %w", err)
		logger.Err(err).Msg("Failed to write patched config map into temporary file")
		tracker.Diagnostics.Push(err)
		return
	}
	if n != int64(len(patchedConfigMap)) {
		err := fmt.Errorf("failed to fully write contents to temporary file")
		logger.Err(err).Msg("Failed to fully write patched config map into temporary file")
		tracker.Diagnostics.Push(err)
		return
	}

	k.Directory = clusterDir
	if err = k.KubectlApply(filepath.Base(file.Name()), "-n kube-public"); err != nil {
		logger.Err(err).Msg("Failed to apply newly patched cluster-info config map")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msgf("Cluster-info Config Map patched successfully")
}
