package service

import (
	"context"
	"fmt"

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
		Result      *spec.TaskResult
		Diagnostics *Diagnostics
	}

	Diagnostics []string

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

func (d *Diagnostics) Push(val string) { (*d) = append(*d, val) }
func (d *Diagnostics) String() string  { return fmt.Sprint(*d) }

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
		case spec.StageAnsibler_DETERMINE_API_ENDPOINT_CHANGE:
			MoveApiEndpoint(logger, work.InputManifestName, processlimit, work.Task, tracker)
		case spec.StageAnsibler_INSTALL_NODE_REQUIREMENTS:
			InstallNodeRequirements(logger, work.InputManifestName, processlimit, work.Task, tracker)
		case spec.StageAnsibler_INSTALL_VPN:
			InstallVPN(logger, work.InputManifestName, processlimit, work.Task, tracker)
		case spec.StageAnsibler_RECONCILE_LOADBALANCERS:
			ReconcileLoadBalancers(logger, work.InputManifestName, processlimit, work.Task, tracker)
		case spec.StageAnsibler_REMOVE_CLAUDIE_UTILITIES:
			logger.Info().Msg("Removing Claudie utilities")
			panic(tracker)
		case spec.StageAnsibler_UPDATE_API_ENDPOINT:
			logger.Info().Msg("Updating API endpoint")
			panic(tracker)
		case spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES:
			logger.Info().Msg("Updating Proxy Envs on cluster nodes")
			panic(tracker)
		case spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES_FOR_K8S:
			logger.Info().Msg("Updating Proxy Envs on cluster nodes for kuberentes services")
			panic(tracker)
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
