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

const (
	// Output directory where generated yaml are generated to.
	OutputDir = "services/kuber/clusters"
)

type (
	Work struct {
		InputManifestName string
		Task              *spec.Task

		// Maximum number of workers a single work group
		// can have.
		WorkersLimit int

		// Passes are the individual transformations
		// that should be done with the task.
		Passes []*spec.StageKuber_SubPass
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
		logger := logger.With().Str("kuber-stage", pass.Kind.String()).Logger()

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
		case spec.StageKuber_CILIUM_RESTART:
			CiliumRolloutRestart(logger, tracker)
		case spec.StageKuber_DELETE_NODES:
			DeleteNodes(logger, tracker)
		case spec.StageKuber_PATCH_CLUSTER_INFO_CM:
			PatchClusterInfoCM(logger, tracker)
		case spec.StageKuber_PATCH_KUBEADM:
			PatchKubeadmCM(logger, tracker)
		case spec.StageKuber_PATCH_KUBE_PROXY:
			PatchKubeProxy(logger, tracker)
		case spec.StageKuber_PATCH_NODES:
			PatchNodes(logger, processlimit, work.WorkersLimit, tracker)
		case spec.StageKuber_REMOVE_LB_SCRAPE_CONFIG:
			RemoveScrapeConfig(logger, tracker)
		case spec.StageKuber_DEPLOY_LONGHORN:
			DeployLonghorn(logger, tracker)
		case spec.StageKuber_ENABLE_LONGHORN_CA:
			EnableLonghornCA(logger, tracker)
		case spec.StageKuber_DISABLE_LONGHORN_CA:
			DisableLonghornCA(logger, tracker)
		case spec.StageKuber_RECONCILE_LONGHORN_STORAGE_CLASSES:
			ReconcileLonghornStorageClasses(logger, tracker)
		case spec.StageKuber_STORE_LB_SCRAPE_CONFIG:
			StoreScrapeConfig(logger, tracker)
		case spec.StageKuber_DEPLOY_KUBELET_CSR_APPROVER:
			DeployKubeletCSRApprover(logger, work.InputManifestName, tracker)
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
