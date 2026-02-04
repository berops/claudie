package service

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/processlimit"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/semaphore"
)

type (
	Work struct {
		InputManifestName string
		Task              *spec.Task

		// Passes are the individual transformations
		// that should be done with the task.
		Passes []*spec.StageKubeEleven_SubPass
	}

	Tracker struct {
		// [Work.Task] worked on.
		Task *spec.Task

		// Result of the [Work.Task] as it is processed by the pipeline.
		Result *spec.TaskResult

		// Diagnostics during the processing of the received [Work.Task]
		Diagnostics *Diagnostics
	}

	Diagnostics []error
)

func (d *Diagnostics) Push(err error) { (*d) = append(*d, err) }

func ProcessTask(ctx context.Context, work Work) *spec.TaskResult {
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
		logger := logger.With().Str("kube-eleven-stage", pass.Kind.String()).Logger()

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
		case spec.StageKubeEleven_DESTROY_CLUSTER:
			Destroy(logger, work.InputManifestName, processlimit, tracker)
		case spec.StageKubeEleven_RECONCILE_CLUSTER:
			Reconcile(logger, work.InputManifestName, processlimit, tracker)
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

				logger.
					Warn().
					Msgf("Ignoring the following diagnostics: %v", diags[last:])

				// zero-out the ignored diagnostics.
				clear(diags[last:])
				diags = diags[:last]

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
