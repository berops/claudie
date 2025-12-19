package service

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
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
		Task              *spec.Task

		// Passes are the individual transformations
		// that should be done with the task.
		Passes []*spec.StageAnsibler_SubPass
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
		case spec.StageAnsibler_CLEAR_PROXY_ENVS_ON_NODES:
			ClearProxyEnvs(logger, work.InputManifestName, processlimit, tracker)
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

// For the update task, looks inside the Delta and if its load balancer related, returns only
// the loadbalancer for which the task is to be processed for, if not found returns nil.
func DefaultToSingleLoadBalancerIfPossible(task *spec.Task_Update) *spec.LBcluster {
	lbs := task.Update.State.LoadBalancers

	var targetHandle string

	switch delta := task.Update.Delta.(type) {
	case *spec.Update_AddedLoadBalancerNodes_:
		targetHandle = delta.AddedLoadBalancerNodes.Handle
	case *spec.Update_AddedLoadBalancerRoles_:
		targetHandle = delta.AddedLoadBalancerRoles.Handle
	case *spec.Update_AddedLoadBalancer_:
		targetHandle = delta.AddedLoadBalancer.Handle
	case *spec.Update_AnsReplaceTargetPools:
		targetHandle = delta.AnsReplaceTargetPools.Handle
	case *spec.Update_DeleteLoadBalancerNodes_:
		targetHandle = delta.DeleteLoadBalancerNodes.Handle
	case *spec.Update_DeleteLoadBalancerRoles_:
		targetHandle = delta.DeleteLoadBalancerRoles.Handle
	case *spec.Update_DeleteLoadBalancer_:
		targetHandle = delta.DeleteLoadBalancer.Handle
	case *spec.Update_ReplacedDns_:
		targetHandle = delta.ReplacedDns.Handle
	case *spec.Update_ReplacedTargetPools_:
		targetHandle = delta.ReplacedTargetPools.Handle
	}

	if targetHandle == "" {
		return nil
	}

	idx := clusters.IndexLoadbalancerById(targetHandle, lbs)
	if idx < 0 {
		return nil
	}

	return lbs[idx]
}

// If the task is related to adding new nodes, this function will return
// a shallow copy of the nodepool with only the new additions. The original
// nodepool is not modified. All of the fields of the new shallow copy are still
// shared with the original nodepool.
//
// **Caution** the counts of the nodes are not changed, thus
//
// If the function can't default to new nodes only, nil is returned.
func DefaultKubernetesToNewNodesIfPossible(task *spec.Task_Update) *spec.NodePool {
	var (
		npId  string
		nodes []string
	)

	switch delta := task.Update.Delta.(type) {
	case *spec.Update_AddedK8SNodes_:
		npId = delta.AddedK8SNodes.Nodepool
		nodes = delta.AddedK8SNodes.Nodes
	case *spec.Update_DeleteK8SNodes_:
		npId = delta.DeleteK8SNodes.Nodepool
		nodes = delta.DeleteK8SNodes.Nodes
	}

	if npId == "" || len(nodes) == 0 {
		return nil
	}

	np := nodepools.FindByName(npId, task.Update.State.K8S.ClusterInfo.NodePools)
	if np == nil {
		return nil
	}
	return nodepools.PartialCopyWithNodeFilter(np, nodes)
}

// Same as [DefaultKubernetesToNewNodesIfPossible] but for load balancer clusters.
func DefaultLoadBalancerToNewNodesIfPossible(task *spec.Task_Update) *spec.NodePool {
	var (
		targetHandle string
		npId         string
		nodes        []string
	)

	switch delta := task.Update.Delta.(type) {
	case *spec.Update_AddedLoadBalancerNodes_:
		targetHandle = delta.AddedLoadBalancerNodes.Handle
		npId = delta.AddedLoadBalancerNodes.NodePool
		nodes = delta.AddedLoadBalancerNodes.Nodes
	case *spec.Update_DeleteLoadBalancerNodes_:
		targetHandle = delta.DeleteLoadBalancerNodes.Handle
		npId = delta.DeleteLoadBalancerNodes.Nodepool
		nodes = delta.DeleteLoadBalancerNodes.Nodes
	}

	if targetHandle == "" || npId == "" || len(nodes) == 0 {
		return nil
	}

	idx := clusters.IndexLoadbalancerById(targetHandle, task.Update.State.LoadBalancers)
	if idx < 0 {
		return nil
	}

	np := nodepools.FindByName(npId, task.Update.State.LoadBalancers[idx].ClusterInfo.NodePools)
	if np == nil {
		return nil
	}

	return nodepools.PartialCopyWithNodeFilter(np, nodes)
}
