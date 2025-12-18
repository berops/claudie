package service

import (
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"

	"gopkg.in/yaml.v3"
)

// Number of retries to perform to try to unmarshal the kubeadm config map
// before giving up.
const ReadKubeadmConfigRetries = 3

func PatchKubeadmCM(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Patching kubeadm config-map")

	action, ok := tracker.Task.Do.(*spec.TaskV2_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task %T while wanting to update kubeadm config map,"+
				" assuming the task was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	var lbApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpointV2(action.Update.State.LoadBalancers); ep != nil {
		lbApiEndpoint = ep.Dns.Endpoint
	}

	certSANs := []string{lbApiEndpoint}
	if lbApiEndpoint == "" {
		certSANs = certSANs[:len(certSANs)-1]
		for n := range nodepools.Control(action.Update.State.K8S.ClusterInfo.NodePools) {
			for _, n := range n.Nodes {
				certSANs = append(certSANs, n.Public)
			}
		}
	}

	// Kubeadm uses this config map when joining new nodes, we need to update it with correct certSANs
	// after api endpoint change.
	// https://github.com/berops/claudie/issues/1597

	k := kubectl.Kubectl{
		Kubeconfig:        action.Update.State.K8S.Kubeconfig,
		MaxKubectlRetries: 3,
	}

	var err error
	var configMap []byte
	var rawKubeadmConfigMap map[string]any

	for i := range ReadKubeadmConfigRetries {
		if i > 0 {
			wait := time.Duration(150+rand.IntN(300)) * time.Millisecond
			logger.Warn().Msgf("reading kubeadm-config failed err: %v, retrying again in %s ms [%v/%v]",
				err,
				wait,
				i+1,
				ReadKubeadmConfigRetries,
			)
			time.Sleep(wait)
		}

		configMap, err = k.KubectlGet("cm kubeadm-config", "-oyaml", "-n kube-system")
		if err != nil || len(configMap) == 0 {
			continue
		}
		if err = yaml.Unmarshal(configMap, &rawKubeadmConfigMap); err != nil {
			continue
		}
		break
	}

	if err != nil {
		err := fmt.Errorf("failed to retrieve kubeadm-config map after %v retries: %w", ReadKubeadmConfigRetries, err)
		logger.Err(err).Msg("Failed to retrieve kubeadm config map")
		tracker.Diagnostics.Push(err)
		return
	}

	if len(configMap) == 0 {
		logger.Warn().Msgf("kubeadm-config config map was not found, skip patching kubeadm-config map")
		return
	}

	data, ok := rawKubeadmConfigMap["data"].(map[string]any)
	if !ok {
		err := fmt.Errorf("expected 'data' field to present, but was missing inside the kubeadm config map")
		logger.Err(err).Msg("Unexpected config map structure")
		tracker.Diagnostics.Push(err)
		return
	}

	config, ok := data["ClusterConfiguration"].(string)
	if !ok {
		err := fmt.Errorf("expected 'ClusterConfiguration' field to present, but was missing inside the kubeadm config map")
		logger.Err(err).Msg("Unexpected config map structure")
		tracker.Diagnostics.Push(err)
		return
	}

	var rawKubeadmConfig map[string]any
	if err := yaml.Unmarshal([]byte(config), &rawKubeadmConfig); err != nil {
		err := fmt.Errorf("failed to unmarshal 'ClusterConfiguration' inside kubeadm-config config map: %w", err)
		logger.Err(err).Msg("Unexpected config map structure")
		tracker.Diagnostics.Push(err)
		return
	}

	apiServer, ok := rawKubeadmConfig["apiServer"].(map[string]any)
	if !ok {
		err := fmt.Errorf("expected 'apiServer' field to present in the 'ClusterConfiguration' for kubeadm, but was missing: %w", err)
		logger.Err(err).Msg("Unexpected config map structure")
		tracker.Diagnostics.Push(err)
		return
	}

	apiServer["certSANs"] = certSANs

	b, err := yaml.Marshal(rawKubeadmConfig)
	if err != nil {
		err := fmt.Errorf("failed to marshal updated 'ClusterConfiguration' for kubeadm: %w", err)
		logger.Err(err).Msg("Marshalling failed")
		tracker.Diagnostics.Push(err)
		return
	}

	data["ClusterConfiguration"] = string(b)

	b, err = yaml.Marshal(rawKubeadmConfigMap)
	if err != nil {
		err := fmt.Errorf("failed to marshal updated kubeadm-config config map: %w", err)
		logger.Err(err).Msg("Marshalling failed")
		tracker.Diagnostics.Push(err)
		return
	}

	if err := k.KubectlApplyString(string(b), "-n kube-system"); err != nil {
		logger.Err(err).Msg("Failed to apply kubeadm config map with new changes")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msgf("Kubeadm-config Config Map patched successfully")
}
