package service

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/processlimit"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/store"
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
		Result      *spec.TaskResult
		Diagnostics *Diagnostics
	}

	Diagnostics []string
)

// TODO: make this more ergonomic.
func (d *Diagnostics) Push(val string) { (*d) = append(*d, val) }
func (d *Diagnostics) String() string  { return fmt.Sprint(*d) }

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
			diags.Push(err.Error())
			break passes
		default:
		}

		tracker := Tracker{
			Result:      &result,
			Diagnostics: &diags,
		}
		last := len(diags)

		switch pass.Kind {
		case spec.StageTerraformer_BUILD_INFRASTRUCTURE:
			logger.Info().Msg("Bulding infrastructure")
			build(logger, work.InputManifestName, processlimit, work.Task, tracker)
		case spec.StageTerraformer_UPDATE_INFRASTRUCTURE:
			logger.Info().Msg("Updating infrastructure")
			reconcileInfrastructure(logger, stores, work.InputManifestName, processlimit, work.Task, tracker)
		case spec.StageTerraformer_DESTROY_INFRASTRUCTURE:
			logger.Info().Msg("Destroying infrastructure")
			destroy(logger, stores, work.InputManifestName, processlimit, work.Task, tracker)
		case spec.StageTerraformer_API_PORT_ON_KUBERNETES:
			logger.Info().Msg("Reconciling Api Port on kuberentes cluster")
			reconcileApiPort(logger, work.InputManifestName, processlimit, work.Task, tracker)
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
