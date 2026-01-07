package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/processlimit"
	"github.com/berops/claudie/proto/pb/spec"
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

	NodePools struct {
		Dynamic []*spec.NodePool
		Static  []*spec.NodePool
	}

	// Utility types while processing the messages.
	NodepoolsInfo struct {
		Nodepools      NodePools
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
	case *spec.Update_DeletedLoadBalancerNodes_:
		targetHandle = delta.DeletedLoadBalancerNodes.Handle
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
// shared with the original nodepool. The shallow copy will have its node count
// adjusted to reflect the filtered out nodes, use with **caution**.
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

// If the task is related to deleted nodes within the kubernetes cluster, will return
// a shallow copy of the affected nodepool with only the deleted nodes. The original
// nodepool is not modified. All of the fields of the new shallow copy are still shared
// with the original nodepool. The shallow copy will have its node count adjusted to
// reflect the filtered out nodes, use with **caution**.
//
// If the function can't default to the deleted nodes, nil is returned.
func DefaultKubernetesToDeletedNodesOnly(k8s *spec.K8Scluster, del *spec.Update_DeletedK8SNodes) *spec.NodePool {
	var n *spec.NodePool
	switch kind := del.Kind.(type) {
	case *spec.Update_DeletedK8SNodes_Partial_:
		np := nodepools.FindByName(kind.Partial.Nodepool, k8s.ClusterInfo.NodePools)
		if np == nil {
			break
		}
		n = nodepools.PartialCopyWithReplacedNodes(np, kind.Partial.Nodes, kind.Partial.StaticNodeKeys)
	case *spec.Update_DeletedK8SNodes_Whole:
		n = kind.Whole.Nodepool
	}
	return n
}

// Same as [DefaultKubernetesToDeletedNodesOnly] but with loadbalancer deletions.
func DefaultLoadBalancerToDeletedNodesOnly(lb *spec.LBcluster, del *spec.Update_DeletedLoadBalancerNodes) *spec.NodePool {
	var n *spec.NodePool
	switch kind := del.Kind.(type) {
	case *spec.Update_DeletedLoadBalancerNodes_Partial_:
		np := nodepools.FindByName(kind.Partial.Nodepool, lb.ClusterInfo.NodePools)
		if np == nil {
			break
		}
		n = nodepools.PartialCopyWithReplacedNodes(np, kind.Partial.Nodes, kind.Partial.StaticNodeKeys)
	case *spec.Update_DeletedLoadBalancerNodes_Whole:
		n = kind.Whole.Nodepool
	}
	return n
}

// Goes over the unreachable nodepools and their nodes and filters them out from the passed in nodepools.
// The original nodepools are not modified in any way, shallow copies are made that only contain reachable
// nodes, which are then returned. The returned nodepools still share memory with the original nodepools as
// only a shallow copy is made. The copies will have their node counts adjusted to reflect the filtered out
// nodes, if any. Use with **caution**.
//
// The return is always non-nil here, if there are is no unreachable infrastructure simply unmodified shallow
// copies of the passed in nodepools are returned.
func DefaultNodePoolsToReachableInfrastructureOnly(
	nps []*spec.NodePool,
	unreachable *spec.Unreachable_UnreachableNodePools,
) []*spec.NodePool {
	var result []*spec.NodePool

	for _, np := range nps {
		unreachable := unreachable.GetNodepools()[np.Name].GetNodes()
		var reachable []string

		for _, n := range np.Nodes {
			if !slices.Contains(unreachable, n.Name) {
				reachable = append(reachable, n.Name)
			}
		}

		cpy := nodepools.PartialCopyWithNodeFilter(np, reachable)
		result = append(result, cpy)
	}

	return result
}

// Looks into the update type of the message and if it has possible unreachable
// infrastructure attached to it returns it. Otherwise nil is returned
func UnreachableInfrastructure(u *spec.Task_Update) *spec.Unreachable {
	switch delta := u.Update.Delta.(type) {
	case *spec.Update_DeletedK8SNodes_:
		return delta.DeletedK8SNodes.Unreachable
	case *spec.Update_DeleteLoadBalancer_:
		return delta.DeleteLoadBalancer.Unreachable
	case *spec.Update_DeletedLoadBalancerNodes_:
		return delta.DeletedLoadBalancerNodes.Unreachable
	default:
		return nil
	}
}
