package service

import (
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
	return nil
}

// Builds the required infrastructure by looking at the difference between
// the current and desired state based on the passed in [loadbalancer.LBcluster].
// On success updates the [loadbalancer.LBcluster.CurrentState] to the desired state.
// On failure, any desred infra is reverted back to current.
func BuildLoadbalancers(logger zerolog.Logger, state loadbalancer.LBcluster) error {
	logger.Info().Msg("Creating loadbalancer infrastructure")

	if err := state.Build(logger); err != nil {
		logger.Err(err).Msg("failed to build cluster")
		return err
	}

	logger.Info().Msg("Loadbalancer infrastructure successfully created")
	return nil
}
