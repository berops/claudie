package service

import (
	"fmt"
	"time"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/managerv2/internal/service/managementcluster"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ScheduleResult describes what has happened during the
// scheduling of the tasks.
type ScheduleResult uint8

// TODO: endless reconciliation...
const (
	// Reschedule describes the case where the manifest should be rescheduled again
	// after either error-ing or completing.
	Reschedule ScheduleResult = iota

	// NoReschedule describes the case where the manifest should not be rescheduled again
	// after either error-ing or completing.
	NoReschedule

	// Noop describes the case where from the reconciliation of the current and desired
	// state no new tasks were identified thus no changes are to be worked on and no
	// need to update the representation in the external storage.
	Noop

	// NotReady describes the case where the manifest is not ready to be scheduled yet,
	// this is mostly related to the retry policies which can vary. For example if
	// an exponential retry policy is used the manifest will not be ready to be scheduled
	// until the specified number of Tick pass.
	NotReady

	// FinalRetry describes the case where a manifest had a retry policy to retry
	// rescheduling the manifest N times before giving up. FinalRetry states that
	// the manifest should be retried one last time before giving up.
	FinalRetry
)

// Schedules tasks based on the difference between the current and desired state.
// No changes to the passed in values are done. The passed in `desired` and `pending`
// states will not be modified in any way.
func reconciliate(pending *spec.ConfigV2, desired map[string]*spec.ClustersV2) ScheduleResult {
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
			hasInFlightState = state.Task != nil && state.Task.Task != nil

			noop      = isCurrentNil && isDesiredNil && !hasInFlightState
			isCreate  = isCurrentNil && !isDesiredNil
			isDestroy = (!isCurrentNil || hasInFlightState) && isDesiredNil
		)

		switch {
		case noop:
			// nothing to do (desired state was not build).
			result = NoReschedule
		case isCreate:
			if hasInFlightState {
				logger.
					Info().
					Msg("Detected cluster to Create, but has previous InFlight state that will be scheduled for deletion")

					// If there is any InFlight state that was not commited
					// to the current state, delete it, as we still don't
					// have any current state and the InFlight state could
					// have been partially applied.
					//
					// If that fails, that will trigger another reconciliation iteration,
					// while keeping the original [state.Current] unmodified.
				state.Task = deleteCluster(state.Task.State)
				break
			}
			state.Task = createCluster(desired)
		case isDestroy:
			if hasInFlightState {
				// If there is any InFlight state that was not commited
				// to the current state, create an Intermediate Representation
				// combining the two together to delete all of the infrastructure.
				// As there could be partiall additions, or partial deletions.
				//
				// If that fails, that will trigger another reconciliation iteration,
				// while keeping the original [state.Current] unmodified.
				ir := clustersUnion(current, state.Task.State)
				state.Task = deleteCluster(ir)
				break
			}

			if err := managementcluster.DeleteKubeconfig(current); err != nil {
				logger.Err(err).Msg("Failed to delete kubeconfig secret in the management cluster")
			}

			if err := managementcluster.DeleteClusterMetadata(current); err != nil {
				logger.Err(err).Msg("Failed to delete metadata secret in the management cluster")
			}

			if current.K8S.AnyAutoscaledNodePools() {
				if err := managementcluster.DestroyClusterAutoscaler(pending.Name, current); err != nil {
					logger.Err(err).Msg("Failed to destroy autoscaler pods")
				}
			}

			state.Task = deleteCluster(current)
		default:
			// TODO: handle this at this stage instead of the
			// transfer_existing stage.
			// This would Also mean that the desired state should be
			// generated in here more as a "skeleton" and then the
			// state should be transfered from current.
			// if desired.AutoscalerConfig != nil {
			// 	switch {
			// 	case desired.AutoscalerConfig.Min > current.Count:
			// 		dnp.Count = dnp.AutoscalerConfig.Min
			// 	case desired.AutoscalerConfig.Max < current.Count:
			// 		dnp.Count = dnp.AutoscalerConfig.Max
			// 	default:
			// 		dnp.Count = cnp.Count
			// 	}
			// }
			if err := managementcluster.StoreKubeconfig(pending.Name, current); err != nil {
				logger.Err(err).Msg("Failed to store kubeconfig in the management cluster")
			}

			if err := managementcluster.StoreClusterMetadata(pending.Name, current); err != nil {
				logger.
					Err(err).
					Msg("Failed to store cluster metadata secret in the management cluster")
			}

			updateAutoscalerPods := state.Current.K8S.AnyAutoscaledNodePools()
			updateAutoscalerPods = updateAutoscalerPods && managementcluster.DriftInAutoscalerPods(pending.Name, current)
			if updateAutoscalerPods {
				if err := managementcluster.SetUpClusterAutoscaler(pending.Name, current); err != nil {
					logger.
						Err(err).
						Msg("Failed to refresh autoscaler pods in the management cluster")
				}
			}

			result = Noop

			work := Diff(current, desired)
			if work == nil {
				break
			}

			state.Task = work
			result = Reschedule
		}

		switch result {
		case Reschedule, NoReschedule, FinalRetry:
			// Events are going to be worked on, thus clear the Error state, if any.
			state.State = &spec.WorkflowV2{
				Status: spec.WorkflowV2_WAIT_FOR_PICKUP,
			}
		case NotReady, Noop:
		}
	}

	return result
}

func PopulateEntriesForNewClusters(
	current *map[string]*spec.ClusterStateV2,
	desired map[string]*spec.ClustersV2,
) {
	if *current == nil {
		*current = make(map[string]*spec.ClusterStateV2)
	}

	for desired := range desired {
		if current := (*current)[desired]; current != nil {
			continue
		}
		// create an entry in the map but without any state at all.
		(*current)[desired] = &spec.ClusterStateV2{
			Current: &spec.ClustersV2{
				K8S:           nil,
				LoadBalancers: &spec.LoadBalancersV2{},
			},
			State: nil,
			Task:  nil,
		}
	}
}

func createCluster(desired *spec.ClustersV2) *spec.TaskEventV2 {
	// Choose initial api endpoint.
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

	pipeline := []*spec.Stage{
		{
			StageKind: &spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Creating infrastructure for the new cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_BUILD_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      "Building desired state infrastructure",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		},
		{
			StageKind: &spec.Stage_Ansibler{
				Ansibler: &spec.StageAnsibler{
					Description: &spec.StageDescription{
						About:      "Configuring newly spawned cluster infrastructure",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageAnsibler_SubPass{
						{
							Kind: spec.StageAnsibler_INSTALL_NODE_REQUIREMENTS,
							Description: &spec.StageDescription{
								About:      "Installing pre-requisites on all of the nodes of the cluster",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageAnsibler_INSTALL_VPN,
							Description: &spec.StageDescription{
								About:      "Setting up VPN across the nodes of the kuberentes and loadbalancer clusters",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
							Description: &spec.StageDescription{
								About:      "Reconciling Envoy service across the loadbalancer nodes",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		},
		{
			StageKind: &spec.Stage_KubeEleven{
				KubeEleven: &spec.StageKubeEleven{
					Description: &spec.StageDescription{
						About:      "Building kubernetes cluster out of the spawned infrastructure",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageKubeEleven_SubPass{
						{
							Kind: spec.StageKubeEleven_RECONCILE_CLUSTER,
							Description: &spec.StageDescription{
								About:      "Creating kubernetes cluster from the set up infrastructure",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		},

		// TODO: will we need this ?
		// This could be handled by the reconciliation loop ??
		// {
		// 	StageKind: &spec.Stage_Kuber{
		// 		Kuber: &spec.StageKuber{
		// 			Description: &spec.StageDescription{
		// 				About:      "Finalizing cluster configuration",
		// 				ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
		// 			},
		// 		},
		// 	},
		// },
	}

	var (
		inFlightState = proto.Clone(desired).(*spec.ClustersV2)
		createK8s     = proto.Clone(desired.GetK8S()).(*spec.K8SclusterV2)
		createLbs     = proto.Clone(desired.GetLoadBalancers()).(*spec.LoadBalancersV2)
		createOp      = spec.CreateV2{
			K8S:           createK8s,
			LoadBalancers: createLbs.GetClusters(),
		}
	)
	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_CREATE_V2,
		State:     inFlightState,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Create{
				Create: &createOp,
			},
		},
		Description: "creating cluster",
		Pipeline:    pipeline,
	}
}

func deleteCluster(current *spec.ClustersV2) *spec.TaskEventV2 {
	var pipeline []*spec.Stage

	if static := nodepools.Static(current.K8S.ClusterInfo.NodePools); len(static) > 0 {
		// The idea is to continue during the destruction of these two stages even if the
		// kube-eleven stage fails. The static nodes could already be unreachable, for
		// example when credits on a provider expired and there is no way to reach those
		// VMs anymore.
		ke := &spec.Stage{
			StageKind: &spec.Stage_KubeEleven{
				KubeEleven: &spec.StageKubeEleven{
					Description: &spec.StageDescription{
						About:      "Destroying kubernetes cluster and related binaries",
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
				},
			},
		}

		ans := &spec.Stage{
			StageKind: &spec.Stage_Ansibler{
				Ansibler: &spec.StageAnsibler{
					Description: &spec.StageDescription{
						About:      "Removing claudie installed utilities across nodes",
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
				},
			},
		}

		pipeline = append(pipeline, ke)
		pipeline = append(pipeline, ans)
	}

	tf := &spec.Stage{
		StageKind: &spec.Stage_Terraformer{
			Terraformer: &spec.StageTerraformer{
				Description: &spec.StageDescription{
					About:      "Destroying infrastructure of the cluster",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageTerraformer_SubPass{
					{
						Kind: spec.StageTerraformer_DESTROY_INFRASTRUCTURE,
						Description: &spec.StageDescription{
							About:      "Destroying current state",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		},
	}

	// TODO: will we need this ?
	// This could be handled by a proper reconciliation loop reading the current state.
	// kb := &spec.Stage{
	// 	StageKind: &spec.Stage_Kuber{
	// 		Kuber: &spec.StageKuber{
	// 			Description: &spec.StageDescription{
	// 				About:      "Cleanup cluster resources in the Claudie Management Cluster",
	// 				ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
	// 			},
	// 		},
	// 	},
	// }

	pipeline = append(pipeline, tf)
	// pipeline = append(pipeline, kb)

	var (
		inFlightState = proto.Clone(current).(*spec.ClustersV2)
		deleteK8s     = proto.Clone(current.GetK8S()).(*spec.K8SclusterV2)
		deleteLbs     = proto.Clone(current.GetLoadBalancers()).(*spec.LoadBalancersV2)
		deleteOp      = spec.DeleteV2_Clusters_{
			Clusters: &spec.DeleteV2_Clusters{
				K8S:           deleteK8s,
				LoadBalancers: deleteLbs.GetClusters(),
			},
		}
	)

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_DELETE_V2,
		State:     inFlightState,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Delete{
				Delete: &spec.DeleteV2{
					Op: &deleteOp,
				},
			},
		},
		Description: "deleting cluster and its attached loadbalancers",
		Pipeline:    pipeline,
	}
}

// Creates an union that is the combination of the two passed in states.
// The returned union does not point to or share any memory with the two passed in states.
func clustersUnion(old, modified *spec.ClustersV2) *spec.ClustersV2 {
	// should be enough to test the k8s cluster presence, as there are
	// no loadbalancers without a kubernetes cluster.
	switch {
	case old.GetK8S() == nil && modified.GetK8S() != nil:
		return proto.Clone(modified).(*spec.ClustersV2)
	case old.GetK8S() != nil && modified.GetK8S() == nil:
		return proto.Clone(old).(*spec.ClustersV2)
	case old.GetK8S() == nil && modified.GetK8S() == nil:
		return &spec.ClustersV2{
			K8S:           nil,
			LoadBalancers: &spec.LoadBalancersV2{},
		}
	}

	var (
		ir = proto.Clone(old).(*spec.ClustersV2)

		odynamic, ostatic = NodePoolsView(ir.GetK8S().GetClusterInfo())
		ndynamic, nstatic = NodePoolsView(modified.GetK8S().GetClusterInfo())

		dynamicDiff = NodePoolsDiff(odynamic, ndynamic)
		staticDiff  = NodePoolsDiff(ostatic, nstatic)

		loadbalancerDiff = LoadBalancersDiff(ir.LoadBalancers, modified.LoadBalancers)
	)

	// 1. Add any nodepools/nodes.
	for np, nodes := range dynamicDiff.PartiallyAdded {
		src := nodepools.FindByName(np, modified.K8S.ClusterInfo.NodePools)
		dst := nodepools.FindByName(np, ir.K8S.ClusterInfo.NodePools)
		nodepools.CopyNodes(dst, src, nodes)
	}

	for np := range dynamicDiff.Added {
		np := nodepools.FindByName(np, modified.K8S.ClusterInfo.NodePools)
		nnp := proto.Clone(np).(*spec.NodePool)
		ir.K8S.ClusterInfo.NodePools = append(ir.K8S.ClusterInfo.NodePools, nnp)
	}

	for np, nodes := range staticDiff.PartiallyAdded {
		src := nodepools.FindByName(np, modified.K8S.ClusterInfo.NodePools)
		dst := nodepools.FindByName(np, ir.K8S.ClusterInfo.NodePools)
		nodepools.CopyNodes(dst, src, nodes)
	}

	for np := range staticDiff.Added {
		np := nodepools.FindByName(np, modified.K8S.ClusterInfo.NodePools)
		nnp := proto.Clone(np).(*spec.NodePool)
		ir.K8S.ClusterInfo.NodePools = append(ir.K8S.ClusterInfo.NodePools, nnp)
	}

	// 2. Same, but for loadbalancers.
	for _, lb := range loadbalancerDiff.Added {
		idx := clusters.IndexLoadbalancerByIdV2(lb, modified.LoadBalancers.Clusters)
		lb := proto.Clone(modified.LoadBalancers.Clusters[idx]).(*spec.LBclusterV2)
		ir.LoadBalancers.Clusters = append(ir.LoadBalancers.Clusters, lb)
	}

	for lb, diff := range loadbalancerDiff.Modified {
		var (
			cidx = clusters.IndexLoadbalancerByIdV2(lb, ir.LoadBalancers.Clusters)
			nidx = clusters.IndexLoadbalancerByIdV2(lb, modified.LoadBalancers.Clusters)

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
					r := proto.Clone(r).(*spec.RoleV2)
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

func Diff(current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	var (
		odynamic, ostatic = NodePoolsView(current.K8S.ClusterInfo)
		ndynamic, nstatic = NodePoolsView(desired.K8S.ClusterInfo)

		k8sDynamicDiff = NodePoolsDiff(odynamic, ndynamic)
		k8sStaticDiff  = NodePoolsDiff(ostatic, nstatic)

		loadbalancersDiff = LoadBalancersDiff(current.LoadBalancers, desired.LoadBalancers)
	)

	if task := lbdiff(current, desired, loadbalancersDiff); task != nil {
		return task
	}

	return k8sdiff(k8sDynamicDiff, k8sStaticDiff)
}

func lbdiff(current, desired *spec.ClustersV2, diff LoadBalancersDiffResult) *spec.TaskEventV2 {
	for _, lb := range diff.Added {
		return joinLoadBalancer(current, desired, lb)
	}
	return nil
}

func joinLoadBalancer(current, desired *spec.ClustersV2, lb string) *spec.TaskEventV2 {
	var (
		idx      = clusters.IndexLoadbalancerByIdV2(lb, desired.LoadBalancers.Clusters)
		toJoin   = proto.Clone(desired.LoadBalancers.Clusters[idx]).(*spec.LBclusterV2)
		inFlight = proto.Clone(current).(*spec.ClustersV2)
		state    = proto.Clone(current).(*spec.ClustersV2)
		updateOp = spec.UpdateV2{
			State: &spec.UpdateV2_State{
				K8S:           state.K8S,
				LoadBalancers: state.LoadBalancers.Clusters,
			},
			Delta: &spec.UpdateV2_JoinLoadBalancer_{
				JoinLoadBalancer: &spec.UpdateV2_JoinLoadBalancer{
					LoadBalancer: toJoin,
				},
			},
		}
	)

	pipeline := []*spec.Stage{
		{
			StageKind: &spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Creating infrastructure for newly added loadbalancer",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      fmt.Sprintf("Spawning infrastructure for %q", lb),
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		},
		{
			StageKind: &spec.Stage_Ansibler{
				Ansibler: &spec.StageAnsibler{
					Description: &spec.StageDescription{
						About:      "Setting up VPN for the newly added loadbalancer",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageAnsibler_SubPass{
						{
							Kind: spec.StageAnsibler_INSTALL_VPN,
							Description: &spec.StageDescription{
								About:      fmt.Sprintf("Install VPN and interconnect %q with existing infrastructure", lb),
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
							Description: &spec.StageDescription{
								About:      fmt.Sprintf("Setup envoy for the newly added loadbalancer %q", lb),
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		},
	}

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		State:     inFlight,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &updateOp,
			},
		},
		Description: "Joining loadbalancer into existing infrastructure",
		Pipeline:    pipeline,
	}
}

func k8sdiff(dynamic, static NodePoolsDiffResult) *spec.TaskEventV2 {
	return nil
}
