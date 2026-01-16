package service

import (
	"fmt"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/managerv2/internal/service/managementcluster"
	"github.com/google/uuid"

	"google.golang.org/protobuf/proto"
)

// ScheduleResult describes what has happened during the
// scheduling of the tasks.
type ScheduleResult uint8

// TODO: handle the Deletion error where on Creation it fails and the subsequetn deletion also fails
// as there is not current state and also there is not InFlight state.
//
// Meaning that Creation fails -> then -> Deletion fails -> Panic.
// TODO: handle the Unreachable optional in the relevant pipeline stages.
// Priority.
//
// TODO: endless reconciliation... double check the control flow such that it will never stop reconciling.
// TODO: with a failed inFlight state there needs to be a decision to be made if a task
// with a higher priority is to be scheduled what happens in that case ?
// TODO: maybe restrucurize the workers project directory ?
// TODO: the diff should be always between the committed current state and the desired state
// if we have an inflight state the scheduled task should be merged with that inFlight state.
// But then what if we add a loadbalancer and it fails in the ansible stage and then delete it
// from the desired state ? the diff will miss this...
const (
	// Reschedule describes the case where the manifest should be rescheduled again
	// after either error-ing or completing.
	Reschedule ScheduleResult = iota

	// NoReschedule describes the case where the manifest should not be rescheduled again
	// after either error-ing or completing.
	NoReschedule

	// Noop describes the case where from the reconciliation of the current and desired
	// state no new tasks were identified, or an error occured during the process, thus
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
	var result ScheduleResult

	PopulateEntriesForNewClusters(&pending.Clusters, desired)

	for cluster, state := range pending.Clusters {
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
			result = NoReschedule
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

				// If there is any InFlight state that was not commited
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

				// If there is any InFlight state that was not commited
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
				if err := managementcluster.DestroyClusterAutoscaler(pending.Name, del); err != nil {
					logger.Err(err).Msg("Failed to destroy autoscaler pods")
				}
			}

			state.InFlight = ScheduleDeleteCluster(del)
		default:
			// TODO: add diff with the in-flight state.
			if err := managementcluster.StoreKubeconfig(pending.Name, current); err != nil {
				logger.Err(err).Msg("Failed to store kubeconfig in the management cluster")
			}

			if err := managementcluster.StoreClusterMetadata(pending.Name, current); err != nil {
				logger.
					Err(err).
					Msg("Failed to store cluster metadata secret in the management cluster")
			}

			updateAutoscalerPods := len(nodepools.Autoscaled(state.Current.K8S.ClusterInfo.NodePools)) > 0
			if updateAutoscalerPods {
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

					if err := managementcluster.SetUpClusterAutoscaler(pending.Name, current); err != nil {
						logger.
							Err(err).
							Msg("Failed to refresh autoscaler pods in the management cluster")
					}
				}
			}

			result = Noop

			// TODO: if for example a dead node comes in during the
			// cycle rescheduling in here it will always error out
			// event though the unreachable node could be removed
			// and then this could be retried. There could be added
			// a new field called Previos Retry, or something that would
			// be stored along the task and on successfull processing would
			// check if it exists and replace it.
			if shouldRescheduleInFlight(state.InFlight) {
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

				result = Reschedule
				break
			}

			// After the [HealthCheckStatus] and the [KubernetesDiff], [LoadBalancersDiff]
			// is made all of the current,desired [spec.Clusters] state is considered
			// immutable and is not modified to not invalidate cached indices for the
			// returned diffs.
			logger.Info().Msg("Health checking current state")

			hc := HealthCheck(logger, current)
			diff := Diff(current, desired)

			if state.InFlight = handleUpdate(hc, diff, current, desired); state.InFlight != nil {
				result = Reschedule

				// Before scheduling any new task, healthcheck the reachability of the
				// build infrastructure to avoid any issues with unreachable nodes during
				// the building of the task, in which case this takes a higher priority
				// then the scheduled task.
				rhc, err := HealthCheckNodeReachability(logger, current)
				if err != nil {
					result = NotReady

					state.InFlight = nil
					state.State.Status = spec.Workflow_ERROR
					state.State.Description = fmt.Sprintf(
						"Can't schedule task, failed to determine reachability of nodes of the clusters: %v",
						err,
					)

					logger.Error().Msg(state.State.Description)
					break
				}

				kr := KubernetesUnreachableNodes{
					Hc:          hc,
					Unreachable: rhc,
					Diff:        &diff.Kubernetes,
					State:       state,
					Desired:     desired,
				}

				switch HandleKubernetesUnreachableNodes(logger, kr) {
				case UnreachableNodesModifiedCurrentState:
					logger.
						Debug().
						Msg("Propagating change to current state after static node removal on the kubernetes level")

					state.InFlight = nil
					result = NotReady
					break event_switch
				case UnreachableNodesPropagateError:
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
					logger.
						Debug().
						Msg("Propagating error for unreachable nodes on the kubernetes level without working on any task")

					state.InFlight = nil
					result = NotReady
					break event_switch
				case UnreachableNodesScheduledTask:
					logger.Debug().Msg("Scheduled Task with higher priority for unreachable nodes on the kubernetes level")
					break event_switch
				case UnreachableNodesNoop:
					// Nothing to do.
				}

				// Same as with the kubernetes unreachable nodes.
				lbr := LoadBalancerUnreachableNodes{
					Unreachable: rhc,
					State:       state,
					Desired:     desired,
				}
				switch HandleLoadBalancerUnreachableNodes(logger, lbr) {
				case UnreachableNodesModifiedCurrentState:
					logger.
						Debug().
						Msg("Propagating change to current state after static node removal on the loadbalancer level")

					state.InFlight = nil
					result = NotReady
					break event_switch
				case UnreachableNodesPropagateError:
					logger.
						Debug().
						Msg("Propagating error for unreachable nodes on the loadbalancer level without working on any task")

					state.InFlight = nil
					result = NotReady
					break event_switch
				case UnreachableNodesScheduledTask:
					logger.
						Debug().
						Msg("Scheduled Task with higher priority for unreachable nodes on the loadbalancer level")

					break event_switch
				case UnreachableNodesNoop:
					// Nothing to do.
				}
			}
		}

		switch result {
		case Reschedule, NoReschedule:
			// Events are going to be worked on, thus clear the Error state, if any.
			state.State = &spec.Workflow{
				Status: spec.Workflow_WAIT_FOR_PICKUP,
			}
		case NotReady, Noop:
		}
	}

	return result
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
