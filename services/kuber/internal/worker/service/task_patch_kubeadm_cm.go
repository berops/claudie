package service

import (
	"bytes"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v3"
)

// Number of retries to perform to try to unmarshal the kubeadm config map
// before giving up.
const PatchKubeadmConfigRetries = 3

func PatchKubeadmCM(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Patching kubeadm config-map")

	action, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task %T while wanting to update kubeadm config map,"+
				" assuming the task was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	clusterId := action.Update.State.K8S.ClusterInfo.Id()
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

	var lbApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpoint(action.Update.State.LoadBalancers); ep != nil {
		lbApiEndpoint = ep.Dns.Endpoint
	}

	// Kubeadm uses this config map when joining new nodes, we need to update it with correct certSANs
	// after api endpoint change.
	// https://github.com/berops/claudie/issues/1597
	certSANs := []string{lbApiEndpoint}
	if lbApiEndpoint == "" {
		certSANs = certSANs[:len(certSANs)-1]
		for n := range nodepools.Control(action.Update.State.K8S.ClusterInfo.NodePools) {
			for _, n := range n.Nodes {
				certSANs = append(certSANs, n.Public)
			}
		}
	}

	var err error
	for i := range PatchKubeadmConfigRetries {
		if i > 0 {
			wait := time.Duration(150+rand.IntN(300)) * time.Millisecond
			logger.Warn().Msgf("reading kubeadm-config failed err: %v, retrying again in %s ms [%v/%v]",
				err,
				wait,
				i+1,
				PatchKubeadmConfigRetries,
			)
			time.Sleep(wait)
		}

		var (
			patched []byte
			file    *os.File
			n       int64
		)

		if patched, err = patchKubeadmConfigMap(action.Update.State.K8S.Kubeconfig, certSANs); err != nil {
			logger.Warn().Msgf("failed to patch kubeadm-config config map: %v", err)
			continue
		}

		if file, err = os.CreateTemp(clusterDir, clusterId); err != nil {
			logger.Warn().Msgf("failed to create temporary file: %v", err)
			continue
		}

		n, err = io.Copy(file, bytes.NewReader(patched))
		if err != nil {
			file.Close()
			logger.Warn().Msgf("failed to write contents to temporary file: %v", err)
			continue
		}
		if n != int64(len(patched)) {
			file.Close()
			logger.Warn().Msg("failed to fully write contents to temporary file")
			continue
		}

		k := kubectl.Kubectl{
			Directory:         clusterDir,
			Kubeconfig:        action.Update.State.K8S.Kubeconfig,
			MaxKubectlRetries: -1, // No retries.
		}

		if err = k.KubectlApply(filepath.Base(file.Name()), "-n kube-system"); err != nil {
			file.Close()
			logger.Warn().Msgf("failed to patch kubeadm-config config map: %v", err)
			continue
		}

		file.Close()
		break
	}

	if err != nil {
		err := fmt.Errorf("failed to patch kubeadm-config map after %v retries: %w", PatchKubeadmConfigRetries, err)
		log.Err(err).Msg("Patching kubeadm config map failed")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msgf("Kubeadm-config Config Map patched successfully")
}

func patchKubeadmConfigMap(kubeconfig string, certSANs []string) ([]byte, error) {
	k := kubectl.Kubectl{
		Kubeconfig:        kubeconfig,
		MaxKubectlRetries: 1,
	}

	configMap, err := k.KubectlGet("cm kubeadm-config", "-oyaml", "-n kube-system")
	if err != nil {
		return nil, err
	}

	if len(configMap) == 0 {
		return nil, fmt.Errorf("received empty kubeadm config map")
	}

	var rawKubeadmConfigMap map[string]any
	if err := yaml.Unmarshal(configMap, &rawKubeadmConfigMap); err != nil {
		return nil, err
	}

	data, ok := rawKubeadmConfigMap["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected 'data' field to present, but was missing inside the kubeadm config map")
	}

	config, ok := data["ClusterConfiguration"].(string)
	if !ok {
		return nil, fmt.Errorf("expected 'ClusterConfiguration' field to present, but was missing inside the kubeadm config map")
	}

	var rawKubeadmConfig map[string]any
	if err := yaml.Unmarshal([]byte(config), &rawKubeadmConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal 'ClusterConfiguration' inside kubeadm-config config map: %w", err)
	}

	apiServer, ok := rawKubeadmConfig["apiServer"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected 'apiServer' field to present in the 'ClusterConfiguration' for kubeadm, but was missing: %w", err)
	}

	apiServer["certSANs"] = certSANs
	if _, ok := rawKubeadmConfig["controlPlaneEndpoint"]; ok {
		rawKubeadmConfig["controlPlaneEndpoint"] = net.JoinHostPort(certSANs[0], fmt.Sprint(manifest.APIServerPort))
	}

	b, err := yaml.Marshal(rawKubeadmConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated 'ClusterConfiguration' for kubeadm: %w", err)
	}

	data["ClusterConfiguration"] = string(b)

	return yaml.Marshal(rawKubeadmConfigMap)
}
