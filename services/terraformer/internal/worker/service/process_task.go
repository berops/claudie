package service

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/processlimit"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/berops/claudie/services/terraformer/internal/worker/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

type (
	Work struct {
		InputManifestName string
		Task              *spec.TaskV2

		// Passes are the individual transformations
		// that should be done with the task.
		Passes []*spec.StageTerraformer_SubPass
	}

	Stores struct {
		s3     store.S3StateStorage
		dynamo store.DynamoDB
	}

	Tracker struct {
		// [Work.Task] worked on.
		Task *spec.TaskV2

		// Result of the [Work.Task] as it is processed by the pipeline.
		//
		// Generally for the terraformer service, additions, even if partially
		// succeded ones or failed ones, are fully commited back to the message
		// queue to be picked up by the manager as there currently is no way of
		// tracking partially build changes. By returning the updated state with
		// the newly added items, even if failed, along the message an error is send
		// and the manager should make out the diff to reconciliate back to the correct
		// state by performing deletions, which when picked up by this service will be
		// a noop if they were not build.
		//
		// For deletions, either it succeeds or not, on failure the new state is not
		// reported back to the manager, contrary to the additions. This is so that
		// there is now way currently to know which items were delete and which not
		// thus again, leave the diff for the manager service.
		Result *spec.TaskResult

		// Diagnostics during the processing of the received [Work.Task]
		Diagnostics *Diagnostics
	}

	Diagnostics []error
)

func (d *Diagnostics) Push(err error) { (*d) = append(*d, err) }

func ProcessTask(ctx context.Context, stores Stores, work Work) *spec.TaskResult {
	logger, ok := loggerutils.Value(ctx)
	if !ok {
		// this on should be set, but in case have a default one.
		logger = log.With().Logger()
		logger.Warn().Msg("No logger attached, using default")
	}

	processlimit, ok := processlimit.Value(ctx)
	if !ok {
		logger.Warn().Msg("No process limit found, using default")
		processlimit = semaphore.NewWeighted(5)
	}

	var (
		// diags holds any errors throughout all of the passes.
		diags Diagnostics

		// state the current state of the progress in [Work].
		// Start with None and update result as passes make changes.
		result = spec.TaskResult{Result: &spec.TaskResult_None_{None: new(spec.TaskResult_None)}}
	)

passes:
	for _, pass := range work.Passes {
		logger := logger.With().Str("terraform-stage", pass.Kind.String()).Logger()

		select {
		case <-ctx.Done():
			err := ctx.Err()
			logger.Err(err).Msg("Stopped passing state through passes, context cancelled")
			diags.Push(err)
			break passes
		default:
		}

		tracker := Tracker{
			Task:        work.Task,
			Result:      &result,
			Diagnostics: &diags,
		}
		last := len(diags)

		switch pass.Kind {
		case spec.StageTerraformer_BUILD_INFRASTRUCTURE:
			logger.Info().Msg("Bulding infrastructure")
			build(logger, work.InputManifestName, processlimit, tracker)
		case spec.StageTerraformer_UPDATE_INFRASTRUCTURE:
			logger.Info().Msg("Updating infrastructure")
			reconcileInfrastructure(logger, stores, work.InputManifestName, processlimit, tracker)
		case spec.StageTerraformer_DESTROY_INFRASTRUCTURE:
			logger.Info().Msg("Destroying infrastructure")
			destroy(logger, stores, work.InputManifestName, processlimit, tracker)
		case spec.StageTerraformer_API_PORT_ON_KUBERNETES:
			logger.Info().Msg("Reconciling Api Port on kuberentes cluster")
			reconcileApiPort(logger, work.InputManifestName, processlimit, tracker)
		default:
			logger.Warn().Msg("Stage not recognized, skipping")
			continue
		}

		if current := len(diags); current > last {
			switch pass.Description.ErrorLevel {
			case spec.ErrorLevel_ERROR_FATAL:
				diags := diags[last:]

				logger.
					Err(fmt.Errorf("%v", diags)).
					Msg("Task failed for the current subpass, stopped processing task, propagating error")

				break passes
			case spec.ErrorLevel_ERROR_WARN:
				logger.
					Warn().
					Msg("Task failed for the current subpass, ignoring error and continuing with next")

				continue passes
			}
		}
	}

	if len(diags) > 0 {
		result.Error = &spec.TaskResult_Error{
			Kind:        spec.TaskResult_Error_PARTIAL,
			Description: fmt.Sprint(diags),
		}
	}

	return &result
}

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
