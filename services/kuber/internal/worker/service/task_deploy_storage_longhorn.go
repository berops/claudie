package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	longhornYaml         = "services/kuber/manifests/longhorn.yaml"
	longhornDefaultsYaml = "services/kuber/manifests/claudie-defaults.yaml"
)

const (
	// minVersionForDrainPolicy is the minimum Longhorn version that supports
	// the node-drain-policy value "block-for-eviction-if-contains-last-replica"
	minVersionForDrainPolicy = "v1.9.2"

	// nodeDrainPolicyValue is the value to set for node-drain-policy setting
	nodeDrainPolicyValue = "block-for-eviction-if-contains-last-replica"
)

func DeployLonghorn(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Setting up longhorn for storage")

	var k8s *spec.K8Scluster

	switch do := tracker.Task.Do.(type) {
	case *spec.Task_Create:
		k8s = do.Create.K8S
	case *spec.Task_Update:
		k8s = do.Update.State.K8S
	default:
		logger.
			Warn().
			Msgf("Received task %T while wanting to setup storage, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	k := kubectl.Kubectl{
		Kubeconfig:        k8s.Kubeconfig,
		MaxKubectlRetries: 3,
	}

	k.Stdout = comm.GetStdOut(k8s.ClusterInfo.Id())
	k.Stderr = comm.GetStdErr(k8s.ClusterInfo.Id())

	// Check if Longhorn is already installed and if version is below v1.9.2
	// If so, patch the node-drain-policy setting before upgrading
	currentVersion, err := getCurrentLonghornVersion(k)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get current Longhorn version, proceeding with installation")
	} else if currentVersion != "" {
		isBelow, err := isVersionBelow(currentVersion, minVersionForDrainPolicy)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to compare Longhorn versions, proceeding with installation")
		} else if isBelow {
			log.Info().Msgf("Current Longhorn version %s is below %s, patching node-drain-policy setting", currentVersion, minVersionForDrainPolicy)
			if err := patchNodeDrainPolicy(k, nodeDrainPolicyValue); err != nil {
				log.Warn().Err(err).Msg("Failed to patch node-drain-policy setting, proceeding with installation")
			}
		}
	}

	if err := k.KubectlApply(longhornYaml); err != nil {
		err := fmt.Errorf("error while applying longhorn.yaml: %w", err)
		logger.Err(err).Msg("Failed to deploy longhorn")
		tracker.Diagnostics.Push(err)
		return
	}

	if err := k.KubectlApply(longhornDefaultsYaml); err != nil {
		err := fmt.Errorf("error while applying claudie default settings for longhorn: %w", err)
		logger.Err(err).Msg("Failed to deploy claudie default longhorn settings")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Longhorn successfully set up")
}

// getCurrentLonghornVersion retrieves the currently installed Longhorn version from the cluster.
// Returns empty string if Longhorn is not installed or version cannot be determined.
func getCurrentLonghornVersion(kc kubectl.Kubectl) (string, error) {
	type deploymentJSON struct {
		Items []struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
		} `json:"items"`
	}

	out, err := kc.KubectlGet("deployment", "-n", "longhorn-system", "-l", "app.kubernetes.io/name=longhorn", "-o", "json")
	if err != nil {
		return "", err
	}

	if strings.Contains(string(out), "No resources found") {
		return "", nil
	}

	var parsed deploymentJSON
	if err := json.Unmarshal(out, &parsed); err != nil {
		return "", fmt.Errorf("error unmarshalling longhorn deployment: %w", err)
	}

	if len(parsed.Items) == 0 {
		return "", nil
	}

	version, ok := parsed.Items[0].Metadata.Labels["app.kubernetes.io/version"]
	if !ok {
		return "", nil
	}

	return version, nil
}

// isVersionBelow checks if the given version is below the target version using semver comparison.
// Returns true if version < target, false otherwise.
func isVersionBelow(version, target string) (bool, error) {
	// Handle empty version (Longhorn not installed)
	if version == "" {
		return false, nil
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return false, fmt.Errorf("error parsing version %s: %w", version, err)
	}

	t, err := semver.NewVersion(target)
	if err != nil {
		return false, fmt.Errorf("error parsing target version %s: %w", target, err)
	}

	return v.LessThan(t), nil
}

// patchNodeDrainPolicy patches the node-drain-policy setting to the specified value.
// This is needed for upgrades from Longhorn versions below v1.9.2.
func patchNodeDrainPolicy(kc kubectl.Kubectl, value string) error {
	patchData := fmt.Sprintf(`{"value":"%s"}`, value)
	if err := kc.KubectlPatch("setting.longhorn.io", "node-drain-policy", patchData, "-n", "longhorn-system", "--type=merge"); err != nil {
		return fmt.Errorf("error patching node-drain-policy setting: %w", err)
	}
	log.Info().Msgf("Patched node-drain-policy setting to %s", value)
	return nil
}
