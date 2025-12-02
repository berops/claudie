package service

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/processlimit"
	"github.com/berops/claudie/proto/pb/spec"
	utils "github.com/berops/claudie/services/ansibler/internal/worker/service/internal"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/semaphore"
)

const (
	// Base directory of the ansibler service.
	BaseDirectory = "services/ansibler/"

	// OutputDirectory where ansible playbooks/inventories are generated to.
	OutputDirectory = "clusters"
)

type (
	Work struct {
		InputManifestName string
		Task              *spec.TaskV2

		// Passes are the individual transformations
		// that should be done with the task.
		Passes []*spec.StageAnsibler_SubPass
	}

	Tracker struct {
		// [Work.Task] worked on.
		Task *spec.TaskV2

		// Result of the [Work.Task] as it is processed by the pipeline.
		Result *spec.TaskResult

		// Diagnostics during the processing of the received [Work.Task]
		Diagnostics *Diagnostics
	}

	Diagnostics []error

	// Utility types while processing the messages.
	NodepoolsInfo struct {
		Nodepools      utils.NodePools
		ClusterID      string
		ClusterNetwork string
	}

	AllNodesInventoryData struct {
		NodepoolsInfo []*NodepoolsInfo
	}
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
		logger := logger.With().Str("ansibler-stage", pass.Kind.String()).Logger()

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
		case spec.StageAnsibler_DETERMINE_API_ENDPOINT_CHANGE:
			MoveApiEndpoint(logger, work.InputManifestName, processlimit, tracker)
		case spec.StageAnsibler_INSTALL_NODE_REQUIREMENTS:
			InstallNodeRequirements(logger, work.InputManifestName, processlimit, tracker)
		case spec.StageAnsibler_INSTALL_VPN:
			InstallVPN(logger, work.InputManifestName, processlimit, tracker)
		case spec.StageAnsibler_RECONCILE_LOADBALANCERS:
			ReconcileLoadBalancers(logger, work.InputManifestName, processlimit, tracker)
		case spec.StageAnsibler_REMOVE_CLAUDIE_UTILITIES:
			RemoveUtilities(logger, work.InputManifestName, processlimit, tracker)
		case spec.StageAnsibler_UPDATE_API_ENDPOINT:
			UpdateApiEndpoint(logger, work.InputManifestName, processlimit, tracker)
		case spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES:
			UpdateProxyEnvs(logger, work.InputManifestName, processlimit, tracker)
		case spec.StageAnsibler_COMMIT_PROXY_ENVS:
			CommitProxyEnvs(logger, work.InputManifestName, processlimit, tracker)
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

// If the task is of [spec.Update_AddedLoadBalancer] or [spec.Update_ReconciledLoadBalancer]
// instead of keeping all of the loadbalancers in lbs slices, only the loadbalancer for
// which the reconciliation is called is kept in the lbs slice.
func DefaultToSingleLoadBalancerIfPossible(task *spec.TaskV2, lbs []*spec.LBclusterV2) []*spec.LBclusterV2 {
	if len(lbs) == 0 {
		return lbs
	}

	u, ok := task.Do.(*spec.TaskV2_Update)
	if !ok {
		return lbs
	}

	switch delta := u.Update.Delta.(type) {
	case *spec.UpdateV2_AddedLoadBalancer_:
		if i := clusters.IndexLoadbalancerByIdV2(delta.AddedLoadBalancer.Handle, lbs); i >= 0 {
			lb := lbs[i]
			clear(lbs)
			lbs = lbs[:0]
			return append(lbs, lb)
		}
	case *spec.UpdateV2_ReconciledLoadBalancer_:
		if i := clusters.IndexLoadbalancerByIdV2(delta.ReconciledLoadBalancer.Handle, lbs); i >= 0 {
			lb := lbs[i]
			clear(lbs)
			lbs = lbs[:0]
			return append(lbs, lb)
		}
	}

	return lbs
}
