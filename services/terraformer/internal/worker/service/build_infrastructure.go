package service

import (
	"errors"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/rs/zerolog"
)

// Builds the required infrastructure by looking at the difference between
// the current and desired state based on the passed in [kubernetes.K8Scluster].
// On success updates the [kubernetes.K8Scluster.CurrentState] to the desired state.
// On failure, any desred infra is reverted back to current.
func BuildK8Scluster(logger zerolog.Logger, state kubernetes.K8Scluster) error {
	logger.Info().Msg("Creating infrastructure")

	if err := state.Build(logger); err != nil {
		logger.Err(err).Msg("failed to build cluster")
		return err
	}

	logger.Info().Msg("Cluster build successfully")
	state.UpdateCurrentState()
	return nil
}

// Builds the required infrastructure by looking at the difference between
// the current and desired state based on the passed in [loadbalancer.LBcluster].
// On success updates the [loadbalancer.LBcluster.CurrentState] to the desired state.
// On failure, any desred infra is reverted back to current.
func BuildLoadbalancers(logger zerolog.Logger, state loadbalancer.LBcluster) error {
	logger.Info().Msg("Creating loadbalancer infrastructure")

	if err := state.Build(logger); err != nil {
		logger.Err(err).Msg("Failed to fully build loadbalancer")

		if errors.Is(err, loadbalancer.ErrCreateNodePools) {
			return err
		}

		if errors.Is(err, loadbalancer.ErrCreateDNSRecord) {
			// infrastructure of the loadbalancer was build
			// but the issue was in refreshing/building the
			// DNS, if there is an existing state keep it
			// and do not overwrite with desired state (which failed).
			var dns *spec.DNS
			if state.CurrentState != nil {
				dns = state.CurrentState.Dns
			}
			state.UpdateCurrentState()
			state.CurrentState.Dns = dns
			return err
		}

		return err
	}

	logger.Info().Msg("Loadbalancer infrastructure successfully created")
	state.UpdateCurrentState()
	return nil
}
