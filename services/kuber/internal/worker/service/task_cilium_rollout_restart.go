package service

import (
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
)

func CiliumRolloutRestart(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msgf("Performing a rollout of the cilium daemonset")

	var kubeconfig string

	switch do := tracker.Task.Do.(type) {
	case *spec.TaskV2_Create:
		kubeconfig = do.Create.K8S.Kubeconfig
	case *spec.TaskV2_Update:
		kubeconfig = do.Update.State.K8S.Kubeconfig
	default:
		logger.
			Warn().
			Msgf("Recevied task with action %T while wanting to rollout restart cilium daemonset, assuming task was misscheduled, ignoring", do)
		return
	}

	if kubeconfig == "" {
		logger.
			Warn().
			Msgf("Can't rollout restart cilium daemonset as the kubeconfig is missing, ignoring")
		return
	}

	kc := kubectl.Kubectl{
		Kubeconfig:        kubeconfig,
		MaxKubectlRetries: 5,
	}

	if err := kc.RolloutRestart("daemonset", "cilium", "-n kube-system"); err != nil {
		logger.Err(err).Msg("Failed to rollout restart cilium daemonset")
		tracker.Diagnostics.Push(err)
		return
	}
}
