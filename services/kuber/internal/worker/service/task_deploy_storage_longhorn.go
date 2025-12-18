package service

import (
	"fmt"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
)

const (
	longhornYaml         = "services/kuber/manifests/longhorn.yaml"
	longhornDefaultsYaml = "services/kuber/manifests/claudie-defaults.yaml"
)

func DeployLonghorn(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Setting up longhorn for storage")

	var k8s *spec.K8SclusterV2

	switch do := tracker.Task.Do.(type) {
	case *spec.TaskV2_Create:
		k8s = do.Create.K8S
	case *spec.TaskV2_Update:
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
