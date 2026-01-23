package service

import (
	"fmt"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/service/managementcluster"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"google.golang.org/protobuf/proto"
)

// ScheduleResult describes what has happened during the
// scheduling of the tasks.
type ScheduleResult uint8

const (
	// Reschedule describes the case where the manifest should be rescheduled again
	// after either error-ing or completing.
	Reschedule ScheduleResult = iota

	// NoReschedule describes the case where the manifest should not be rescheduled again
	// after either error-ing or completing.
	NoReschedule

	// Noop describes the case where from the reconciliation of the current and desired
	// state no new tasks were identified, or an error occurred during the process, thus
	// no changes are to be worked on and no need to update the representation in the external
	// storage.
	Noop

	// NotReady describes the case where the manifest is not ready to be scheduled yet,
	// But needs to update its Database representation as changes have been made to it.
	NotReady
)

// Schedules tasks based on the difference between the current and desired state.
// No changes to the passed in values are done. The passed in `desired` and `pending`
// states will not be modified in any way.
func reconciliate(pending *spec.Config, desired map[string]*spec.Clusters) ScheduleResult {
	PopulateEntriesForNewClusters(&pending.Clusters, desired)

	clusterResult := make(map[string]ScheduleResult, len(pending.Clusters))

	for cluster, state := range pending.Clusters {
		// The default settings if always to reschedule
		// until decided to do otherwise.
		clusterResult[cluster] = Reschedule

		var (
			logger = loggerutils.WithProjectAndCluster(pending.Name, cluster)

			// It is guaranteed by validation, that within a single InputManifest
			// no two clusters (including LB) can share the same name.
			current = state.Current
			desired = desired[cluster]

			isCurrentNil     = current == nil || (current.K8S == nil && len(current.LoadBalancers.Clusters) == 0)
			isDesiredNil     = desired == nil || (desired.K8S == nil && len(desired.LoadBalancers.Clusters) == 0)
			hasInFlightState = state.InFlight != nil && state.InFlight.Task != nil

			noop      = isCurrentNil && isDesiredNil && !hasInFlightState
			isCreate  = isCurrentNil && !isDesiredNil
			isDestroy = (!isCurrentNil || hasInFlightState) && isDesiredNil
		)

	event_switch:
		switch {
		case noop:
			// there is no desired state and no current state.
			// the cluster if deleted, stop rescheduling.
			clusterResult[cluster] = NoReschedule
		case isCreate:
			if hasInFlightState {
				logger.
					Info().
					Msg("Detected cluster to Create, but has previous InFlight state that will be scheduled for deletion")

				cs, err := state.InFlight.Task.MutableClusters()
				if err != nil {
					logger.Err(err).Msg("Failed to schedule a destroy of the cluster, skipping")
					break
				}

				// If there is any InFlight state that was not committed
				// to the current state, delete it, as we still don't
				// have any current state and the InFlight state could
				// have been partially applied.
				//
				// If that fails, that will trigger another reconciliation iteration,
				// while keeping the original [state.Current] unmodified.
				state.InFlight = ScheduleDeleteCluster(cs)
				break
			}

			// Choose API endpoint.
			var ep bool
			for _, lb := range desired.GetLoadBalancers().GetClusters() {
				if lb.HasApiRole() {
					lb.UsedApiEndpoint = true
					ep = true
					break
				}
			}
			if !ep {
				nps := desired.K8S.ClusterInfo.NodePools
				nodepools.FirstControlNode(nps).NodeType = spec.NodeType_apiEndpoint
			}

			state.InFlight = ScheduleCreateCluster(desired)
		case isDestroy:
			del := current
			if hasInFlightState {
				cs, err := state.InFlight.Task.MutableClusters()
				if err != nil {
					logger.Err(err).Msg("Failed to schedule a destroy of the cluster, skipping")
					break
				}

				// If there is any InFlight state that was not committed
				// to the current state, create an Intermediate Representation
				// combining the two together to delete all of the infrastructure.
				// As there could be partiall additions, or partial deletions.
				//
				// If that fails, that will trigger another reconciliation iteration,
				// while keeping the original [state.Current] unmodified.
				del = clustersUnion(current, cs)
			}

			if err := managementcluster.DeleteKubeconfig(del); err != nil {
				logger.Err(err).Msg("Failed to delete kubeconfig secret in the management cluster")
			}

			if err := managementcluster.DeleteClusterMetadata(del); err != nil {
				logger.Err(err).Msg("Failed to delete metadata secret in the management cluster")
			}

			if len(nodepools.Autoscaled(del.K8S.ClusterInfo.NodePools)) > 0 {
				if err := managementcluster.DestroyClusterAutoscaler(logger, pending.Name, del); err != nil {
					logger.Err(err).Msg("Failed to destroy autoscaler pods")
				}
			}

			state.InFlight = ScheduleDeleteCluster(del)
		default:
			if err := managementcluster.StoreKubeconfig(pending.Name, current); err != nil {
				logger.Err(err).Msg("Failed to store kubeconfig in the management cluster")
			}

			if err := managementcluster.StoreClusterMetadata(pending.Name, current); err != nil {
				logger.
					Err(err).
					Msg("Failed to store cluster metadata secret in the management cluster")
			}

			currentAutoscaled := len(nodepools.Autoscaled(state.Current.K8S.ClusterInfo.NodePools)) > 0
			if currentAutoscaled {
				drift, err := managementcluster.DriftInAutoscalerPods(logger, pending.Name, current)
				if err != nil {
					logger.
						Err(err).
						Msg("Failed to detect drift in autoscaler deployments")
				}

				if drift {
					logger.
						Info().
						Msg("Detected drift in autoscaler deployments")

					if err := managementcluster.SetUpClusterAutoscaler(logger, pending.Name, current); err != nil {
						logger.
							Err(err).
							Msg("Failed to refresh autoscaler pods in the management cluster")
					}
				}
			} else {
				if err := managementcluster.DestroyClusterAutoscaler(logger, pending.Name, state.Current); err != nil {
					// ignore error.
					log.
						Debug().
						Msgf("Failed to destroy cluster autoscaler pod, could be because current state does not have any autoscaled nodepools: %v", err)
				}
			}

			clusterResult[cluster] = Noop

			// Could be nil or could be a Task that failed.
			lastTask := state.InFlight

			current := current
			desired := desired
			if lastTask != nil {
				inFlight, err := lastTask.Task.MutableClusters()
				if err != nil {
					state.State.Status = spec.Workflow_ERROR
					state.State.Description = fmt.Sprintf(`
Failed to extract state from failed 'InFlight' state: %v
`, err)
					logger.Error().Msg(state.State.Description)
					break event_switch
				}

				// explicitly clone to avoid any unwated modifications.
				inFlight = proto.Clone(inFlight).(*spec.Clusters)

				// If there is an InFlight, make it current and the actual
				// current will be the desired state to roll back to.
				current, desired = inFlight, current
			}

			// After the [HealthCheckStatus] and the [KubernetesDiff], [LoadBalancersDiff]
			// is made all of the current,desired [spec.Clusters] state is considered
			// immutable and is not modified to not invalidate cached indices for the
			// returned diffs.
			logger.Debug().Msg("Health checking current state")

			hc := HealthCheck(logger, current)
			diff := Diff(current, desired)

			if shouldRescheduleInFlight(lastTask) {
				clusterResult[cluster] = Reschedule

				logger.
					Info().
					Msg("Verifying reachability of the cluster before scheduling task to work on")

				// Before scheduling any new task, healthcheck the reachability of the
				// built infrastructure to avoid any issues with unreachable nodes during
				// the building of the task, in which case this takes a higher priority
				// then the scheduled task.
				rhc, err := HealthCheckNodeReachability(logger, current)
				if err != nil {
					clusterResult[cluster] = NotReady

					state.State.Status = spec.Workflow_ERROR
					state.State.Description = fmt.Sprintf(
						"Can't schedule task, failed to determine reachability of nodes of the clusters: %v",
						err,
					)

					logger.Error().Msg(state.State.Description)
					break event_switch
				}

				next, err := handleClusterReachability(logger, current, desired, diff, rhc, hc)
				if err != nil {
					logger.
						Debug().
						Msg("Propagating error for unreachable nodes without working on any task")

					// Higher priority task can't be scheduled due to wait on user
					// input on the unreachable nodes.
					//
					// If the user did not delete the unreachable nodes via kubectl or the user
					// did not remove the whole nodepool with the unreachable nodes from the
					// desired state we cannot proceed further as we need to remove all the
					// nodes with connectivity issues to avoid problems when executing tasks
					// to move the current state towards the desired state, as the workflow will
					// get stuck in ansibler which connects to the nodes via ssh. Thus propagate
					// the error to the user.
					state.State.Status = spec.Workflow_ERROR
					state.State.Description = err.Error()
					logger.Error().Msg(state.State.Description)
					clusterResult[cluster] = NotReady
					break event_switch
				}
				if next != nil {
					logger.
						Info().
						Msg("Scheduled Task with higher priority for unreachable nodes on the kubernetes level")

					next.LowerPriority = lastTask
					state.InFlight = next
					clusterResult[cluster] = Reschedule
					break event_switch
				}

				newUUID := uuid.New().String()

				logger.
					Info().
					Msgf(
						"Rescheduling last failed task %q, Scheduling under new ID %q",
						state.InFlight.Id,
						newUUID,
					)

				// Scheduling under a new UUID is necessary to avoid
				// catching duplicates under NATS when rescheduling.
				state.InFlight.Id = newUUID

				break
			}

			if state.InFlight = handleUpdate(hc, diff, current, desired); state.InFlight != nil {
				clusterResult[cluster] = Reschedule

				logger.
					Info().
					Msg("Verifying reachability of the cluster before scheduling task to work on")

				// Before scheduling any new task, healthcheck the reachability of the
				// build infrastructure to avoid any issues with unreachable nodes during
				// the building of the task, in which case this takes a higher priority
				// then the scheduled task.
				rhc, err := HealthCheckNodeReachability(logger, current)
				if err != nil {
					clusterResult[cluster] = NotReady

					state.State.Status = spec.Workflow_ERROR
					state.State.Description = fmt.Sprintf(
						"Can't schedule task, failed to determine reachability of nodes of the clusters: %v",
						err,
					)

					logger.Error().Msg(state.State.Description)
					break event_switch
				}

				next, err := handleClusterReachability(logger, current, desired, diff, rhc, hc)
				if err != nil {
					clusterResult[cluster] = NotReady

					logger.
						Debug().
						Msg("Propagating error for unreachable nodes without working on any task")

					state.State.Status = spec.Workflow_ERROR
					state.State.Description = err.Error()
					logger.Error().Msg(state.State.Description)
					break event_switch
				}
				if next != nil {
					clusterResult[cluster] = Reschedule

					logger.
						Info().
						Msg("Scheduled Task with higher priority for unreachable nodes on the kubernetes level")

					next.LowerPriority = lastTask
					state.InFlight = next
					break event_switch
				}
			}
		}

		switch clusterResult[cluster] {
		case Reschedule, NoReschedule:
			// Events are going to be worked on, thus clear the Error state, if any.
			state.State = &spec.Workflow{
				Status: spec.Workflow_WAIT_FOR_PICKUP,
			}
		case NotReady, Noop:
		}
	}

	var (
		finalResult ScheduleResult

		reschedule   int
		noReschedule int
		noop         int
		notReady     int
	)

	for _, r := range clusterResult {
		switch r {
		case NoReschedule:
			noReschedule += 1
		case Noop:
			noop += 1
		case NotReady:
			notReady += 1
		case Reschedule:
			reschedule += 1
		}
	}

	switch {
	case reschedule > 0:
		finalResult = Reschedule
	case notReady > 0:
		finalResult = NotReady
	case noReschedule == len(clusterResult):
		finalResult = NoReschedule
	case noop == len(clusterResult):
		finalResult = Noop
	default:
		finalResult = Reschedule
	}

	return finalResult
}

func PopulateEntriesForNewClusters(
	current *map[string]*spec.ClusterState,
	desired map[string]*spec.Clusters,
) {
	if *current == nil {
		*current = make(map[string]*spec.ClusterState)
	}

	for desired := range desired {
		if current := (*current)[desired]; current != nil {
			continue
		}
		// create an entry in the map but without any state at all.
		(*current)[desired] = &spec.ClusterState{
			Current: &spec.Clusters{
				K8S:           nil,
				LoadBalancers: &spec.LoadBalancers{},
			},
			State:    nil,
			InFlight: nil,
		}
	}
}

// Creates an union that is the combination of the two passed in states.
// The returned union does not point to or share any memory with the two passed in states.
// The only "place" where a union is not made is the DNS of a LoadBalancer if it differs
// in the two passed in states, always the on in the old is used.
func clustersUnion(old, modified *spec.Clusters) *spec.Clusters {
	// should be enough to test the k8s cluster presence, as there are
	// no loadbalancers without a kubernetes cluster.
	switch {
	case old.GetK8S() == nil && modified.GetK8S() != nil:
		return proto.Clone(modified).(*spec.Clusters)
	case old.GetK8S() != nil && modified.GetK8S() == nil:
		return proto.Clone(old).(*spec.Clusters)
	case old.GetK8S() == nil && modified.GetK8S() == nil:
		return &spec.Clusters{
			K8S:           nil,
			LoadBalancers: &spec.LoadBalancers{},
		}
	}

	var (
		ir  = proto.Clone(old).(*spec.Clusters)
		k8s = KubernetesDiff(ir.GetK8S(), modified.GetK8S())
		lbs = LoadBalancersDiff(ir.LoadBalancers, modified.LoadBalancers)
	)

	// 1. Add any nodepools/nodes.
	for np, nodes := range k8s.Dynamic.PartiallyAdded {
		src := nodepools.FindByName(np, modified.K8S.ClusterInfo.NodePools)
		dst := nodepools.FindByName(np, ir.K8S.ClusterInfo.NodePools)
		nodepools.CopyNodes(dst, src, nodes)
	}

	for np := range k8s.Dynamic.Added {
		np := nodepools.FindByName(np, modified.K8S.ClusterInfo.NodePools)
		nnp := proto.Clone(np).(*spec.NodePool)
		ir.K8S.ClusterInfo.NodePools = append(ir.K8S.ClusterInfo.NodePools, nnp)
	}

	for np, nodes := range k8s.Static.PartiallyAdded {
		src := nodepools.FindByName(np, modified.K8S.ClusterInfo.NodePools)
		dst := nodepools.FindByName(np, ir.K8S.ClusterInfo.NodePools)
		nodepools.CopyNodes(dst, src, nodes)
	}

	for np := range k8s.Static.Added {
		np := nodepools.FindByName(np, modified.K8S.ClusterInfo.NodePools)
		nnp := proto.Clone(np).(*spec.NodePool)
		ir.K8S.ClusterInfo.NodePools = append(ir.K8S.ClusterInfo.NodePools, nnp)
	}

	// 2. Same, but for loadbalancers.
	for _, lb := range lbs.Added {
		idx := clusters.IndexLoadbalancerById(lb.Id, modified.LoadBalancers.Clusters)
		lb := proto.Clone(modified.LoadBalancers.Clusters[idx]).(*spec.LBcluster)
		ir.LoadBalancers.Clusters = append(ir.LoadBalancers.Clusters, lb)
	}

	for lb, diff := range lbs.Modified {
		var (
			cidx = clusters.IndexLoadbalancerById(lb, ir.LoadBalancers.Clusters)
			nidx = clusters.IndexLoadbalancerById(lb, modified.LoadBalancers.Clusters)

			clb = ir.LoadBalancers.Clusters[cidx]
			nlb = modified.LoadBalancers.Clusters[nidx]
		)

		// Add deleted nodepools
		for np, nodes := range diff.Dynamic.PartiallyAdded {
			src := nodepools.FindByName(np, nlb.ClusterInfo.NodePools)
			dst := nodepools.FindByName(np, clb.ClusterInfo.NodePools)
			nodepools.CopyNodes(dst, src, nodes)
		}

		for np := range diff.Dynamic.Added {
			np := nodepools.FindByName(np, nlb.ClusterInfo.NodePools)
			nnp := proto.Clone(np).(*spec.NodePool)
			clb.ClusterInfo.NodePools = append(clb.ClusterInfo.NodePools, nnp)
		}

		for np, nodes := range diff.Static.PartiallyAdded {
			src := nodepools.FindByName(np, nlb.ClusterInfo.NodePools)
			dst := nodepools.FindByName(np, clb.ClusterInfo.NodePools)
			nodepools.CopyNodes(dst, src, nodes)
		}

		for np := range diff.Static.Added {
			np := nodepools.FindByName(np, nlb.ClusterInfo.NodePools)
			nnp := proto.Clone(np).(*spec.NodePool)
			clb.ClusterInfo.NodePools = append(clb.ClusterInfo.NodePools, nnp)
		}

		// Merge Missing Roles.
		for _, added := range diff.Roles.Added {
			for _, r := range nlb.Roles {
				if r.Name == added {
					r := proto.Clone(r).(*spec.Role)
					clb.Roles = append(clb.Roles, r)
					break
				}
			}
		}

		// Merge Missing TargetPools.
		for role, pools := range diff.Roles.TargetPoolsAdded {
			for _, r := range clb.Roles {
				if r.Name == role {
					r.TargetPools = append(r.TargetPools, pools...)
					break
				}
			}
		}
	}

	return ir
}

// Handles reconciliation for updating existing clusters.
func handleUpdate(hc HealthCheckStatus, diff DiffResult, current, desired *spec.Clusters) *spec.TaskEvent {
	var (
		lbr = LoadBalancersReconciliate{
			Hc:      &hc,
			Diff:    &diff.LoadBalancers,
			Proxy:   &diff.Kubernetes.Proxy,
			Current: current,
			Desired: desired,
		}
		kr = KubernetesReconciliate{
			Hc:      &hc,
			Diff:    &diff.Kubernetes,
			Current: current,
			Desired: desired,
		}
	)

	// NOTE:
	// The follwing are executed in specific order, changing the order may/will
	// affect the outcome of building/reconciling the cluster including lbs.

	if next := PreKubernetesDiff(lbr); next != nil {
		return next
	}

	if next := KubernetesModifications(kr); next != nil {
		return next
	}

	if next := PostKubernetesDiff(lbr); next != nil {
		return next
	}

	if next := KubernetesDeletions(kr); next != nil {
		return next
	}

	if hc.Cluster.VpnDrift {
		return ScheduleRefreshVPN(kr.Diff.Proxy.CurrentUsed, current)
	}

	if next := KubernetesLowPriority(kr); next != nil {
		return next
	}

	return nil
}

// Checks whether the inFlight task should be rescheduled again.
func shouldRescheduleInFlight(inFlight *spec.TaskEvent) bool {
	if inFlight == nil {
		return false
	}

	update, ok := inFlight.Task.Do.(*spec.Task_Update)
	if !ok {
		// only updates inFlights can be rescheduled.
		return false
	}

	// Anything related to deletion, api endpoint or version upgrades
	// needs to be rescheduled again as these tasks cannot be rolled back.
	switch update.Update.Delta.(type) {
	case
		*spec.Update_ApiEndpoint_,

		*spec.Update_DeleteLoadBalancerRoles_,

		*spec.Update_DeleteLoadBalancer_,

		*spec.Update_TfDeleteLoadBalancerNodes,
		*spec.Update_DeletedLoadBalancerNodes_,

		*spec.Update_K8SApiEndpoint,

		*spec.Update_KDeleteNodes,
		*spec.Update_DeletedK8SNodes_,

		*spec.Update_TfReplaceDns,
		*spec.Update_ReplacedDns_,

		*spec.Update_UpgradeVersion_:
		return true
	default:
		return false
	}
}

func handleClusterReachability(
	logger zerolog.Logger,
	current *spec.Clusters,
	desired *spec.Clusters,
	diff DiffResult,
	rhc UnreachableNodes,
	hc HealthCheckStatus,
) (*spec.TaskEvent, error) {
	kr := KubernetesUnreachableNodes{
		Hc:          hc,
		Unreachable: rhc,
		Diff:        &diff.Kubernetes,
		Current:     current,
		Desired:     desired,
	}

	next, err := HandleKubernetesUnreachableNodes(logger, kr)
	if err != nil {
		return nil, err
	}
	if next != nil {
		return next, nil
	}

	// Same as with the kubernetes unreachable nodes.
	lbr := LoadBalancerUnreachableNodes{
		Unreachable: rhc,
		Diff:        &diff.Kubernetes,
		Current:     current,
		Desired:     desired,
	}

	return HandleLoadBalancerUnreachableNodes(lbr)
}
