package service

import (
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

// TODO: kubernetes cluster
// TODO: endless reconciliation... double check the control flow such that it will never stop reconciling.
// TODO: with a failed inFlight state there needs to be a decision to be made if a task
// with a higher priority is to be scheduled what happens in that case ?
// TODO: maybe restrucurize the workers project directory ?
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
			hasInFlightState = state.InFlight != nil && state.InFlight.Task != nil

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

			if nodepools.AnyAutoscaled(del.K8S.ClusterInfo.NodePools) {
				if err := managementcluster.DestroyClusterAutoscaler(pending.Name, del); err != nil {
					logger.Err(err).Msg("Failed to destroy autoscaler pods")
				}
			}

			state.InFlight = ScheduleDeleteCluster(del)
		default:
			// TODO: the autoscaler desired state could be
			// read from the POD directly instead of the
			// pod making requests to the manager. The manager
			// should read the desired state of the autoscaler.
			// errors should be handled gracefully. the autoscaler
			// desired state should be passed when creating our desired state.
			// so that the dynamic nodepools count is correct. But we also need
			// to make sure that in the case the autoscaler is not reachable
			// we will always keep the count from the current state.
			// TODO: add diff with the in-flight state.
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
			// TODO: what if there is always an inflight in-between state ?
			if err := managementcluster.StoreKubeconfig(pending.Name, current); err != nil {
				logger.Err(err).Msg("Failed to store kubeconfig in the management cluster")
			}

			if err := managementcluster.StoreClusterMetadata(pending.Name, current); err != nil {
				logger.
					Err(err).
					Msg("Failed to store cluster metadata secret in the management cluster")
			}

			updateAutoscalerPods := nodepools.AnyAutoscaled(state.Current.K8S.ClusterInfo.NodePools)
			updateAutoscalerPods = updateAutoscalerPods && managementcluster.DriftInAutoscalerPods(pending.Name, current)
			if updateAutoscalerPods {
				if err := managementcluster.SetUpClusterAutoscaler(pending.Name, current); err != nil {
					logger.
						Err(err).
						Msg("Failed to refresh autoscaler pods in the management cluster")
				}
			}

			result = Noop

			// After the [HealthCheckStatus] and the [KubernetesDiff], [LoadBalancersDiff]
			// is made all of the current,desired [spec.Clusters] state is considered
			// immutable and is not modified to not invalidate cached indices for the
			// returned diffs.
			logger.Info().Msg("Health checking current state")
			hc, err := HealthCheck(logger, current)
			if err != nil {
				logger.Err(err).Msg("Failed to fully healthcheck cluster")
				break
			}

			k8s := KubernetesDiff(current.K8S, desired.K8S)
			lbs := LoadBalancersDiff(current.LoadBalancers, desired.LoadBalancers)

			if state.InFlight = PreKubernetesDiff(&hc, &lbs, current, desired); state.InFlight != nil {
				result = Reschedule
				break
			}

			if state.InFlight = KubernetesModifications(&hc, &k8s, current, desired); state.InFlight != nil {
				result = Reschedule
				break
			}

			if state.InFlight = PostKubernetesDiff(&hc, &lbs, current, desired); state.InFlight != nil {
				result = Reschedule
				break
			}

			if state.InFlight = KubernetesDeletions(&hc, &k8s, current, desired); state.InFlight != nil {
				result = Reschedule
				break
			}

			if hc.Cluster.VpnDrift {
				result = Reschedule
				state.InFlight = ScheduleRefreshVPN(current)
			}
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
			State:    nil,
			InFlight: nil,
		}
	}
}

// Schedules a [spec.TaskEvent] task for reconciling the VPN across the nodes of the clusters in the
// passed in [spec.Clusters].
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleRefreshVPN(current *spec.ClustersV2) *spec.TaskEventV2 {
	inFlight := proto.Clone(current).(*spec.ClustersV2)
	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &spec.UpdateV2{
					State: &spec.UpdateV2_State{
						K8S:           inFlight.K8S,
						LoadBalancers: inFlight.LoadBalancers.Clusters,
					},
					Delta: &spec.UpdateV2_None_{},
				},
			},
		},
		Description: "Refreshing VPN",
		Pipeline: []*spec.Stage{
			{
				StageKind: &spec.Stage_Ansibler{
					Ansibler: &spec.StageAnsibler{
						Description: &spec.StageDescription{
							About:      "Configuring infrastructure",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageAnsibler_SubPass{
							{
								Kind: spec.StageAnsibler_INSTALL_VPN,
								Description: &spec.StageDescription{
									About:      "Fixing drift in VPN across nodes of the kuberentes and loadbalancer clusters",
									ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
								},
							},
						},
					},
				},
			},
		},
	}
}

// Schedules a [spec.TaskEvent] task for creating the clusters in the passed in desired [spec.Clusters].
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleCreateCluster(desired *spec.ClustersV2) *spec.TaskEventV2 {
	// Stages
	var (
		tf = spec.Stage_Terraformer{
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
		}

		ansProxy = spec.Stage_Ansibler{
			Ansibler: &spec.StageAnsibler{
				Description: &spec.StageDescription{
					About:      "Configuring newly spawned cluster infrastructure",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageAnsibler_SubPass{
					{
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables based on proxy specification",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
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
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables based on proxy specification, after populating Private addresses",
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
		}

		ansNoProxy = spec.Stage_Ansibler{
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
		}

		kubeeleven = spec.Stage_KubeEleven{
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
		}

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
	)

	var proxy spec.Proxy
	useProxy := desired.K8S.InstallationProxy.Mode == ProxyDefaultMode && hasHetznerNode(desired)
	useProxy = useProxy || desired.K8S.InstallationProxy.Mode == ProxyOnMode
	if useProxy {
		proxy.Op = spec.Proxy_MODIFIED
		proxy.HttpProxyUrl, proxy.NoProxyList = httpProxyUrlAndNoProxyList(desired)
	}

	var (
		createK8s = proto.Clone(desired.GetK8S()).(*spec.K8SclusterV2)
		createLbs = proto.Clone(desired.GetLoadBalancers()).(*spec.LoadBalancersV2)
		createOp  = spec.CreateV2{
			K8S:           createK8s,
			LoadBalancers: createLbs.GetClusters(),
		}
	)

	pipeline := []*spec.Stage{
		{StageKind: &tf},
		{StageKind: nil},
		{StageKind: &kubeeleven},
	}

	if useProxy {
		createOp.Proxy = &proxy
		pipeline[1].StageKind = &ansProxy
	} else {
		pipeline[1].StageKind = &ansNoProxy
	}

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_CREATE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Create{
				Create: &createOp,
			},
		},
		Description: "creating cluster",
		Pipeline:    pipeline,
	}
}

// Schedules a [spec.TaskEvent] task for deleting the clusters in the passed in current [spec.Clusters].
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleDeleteCluster(current *spec.ClustersV2) *spec.TaskEventV2 {
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

	pipeline = append(pipeline, &spec.Stage{
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
	})

	var (
		deleteK8s = proto.Clone(current.GetK8S()).(*spec.K8SclusterV2)
		deleteLbs = proto.Clone(current.GetLoadBalancers()).(*spec.LoadBalancersV2)
		deleteOp  = spec.DeleteV2{
			K8S:           deleteK8s,
			LoadBalancers: deleteLbs.GetClusters(),
		}
	)

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_DELETE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Delete{
				Delete: &deleteOp,
			},
		},
		Description: "deleting cluster and its attached loadbalancers",
		Pipeline:    pipeline,
	}
}

// TODO: the diff should be always between the committed current state and the desired state
// if we have an inflight state the scheduled task should be merged with that inFlight state.
// But then what if we add a loadbalancer and it fails in the ansible stage and then delete it
// from the desired state ? the diff will miss this...

// Creates an union that is the combination of the two passed in states.
// The returned union does not point to or share any memory with the two passed in states.
// The only "place" where a union is not made is the DNS of a LoadBalancer if it differs
// in the two passed in states, always the on in the old is used.
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
		ir  = proto.Clone(old).(*spec.ClustersV2)
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
		idx := clusters.IndexLoadbalancerByIdV2(lb.Id, modified.LoadBalancers.Clusters)
		lb := proto.Clone(modified.LoadBalancers.Clusters[idx]).(*spec.LBclusterV2)
		ir.LoadBalancers.Clusters = append(ir.LoadBalancers.Clusters, lb)
	}

	for lb, diff := range lbs.Modified {
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
