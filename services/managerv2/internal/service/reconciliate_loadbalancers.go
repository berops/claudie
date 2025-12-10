package service

import (
	"fmt"
	"slices"
	"time"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Wraps data and diffs needed by the reconciliation
// for loadbalancers attached to the kubernetes cluster.
//
// The LoadBalancer reconciliation will only consider
// fixing drifts in the passed [LoadBalancerDiffResult]
// Others will only be used for guiding the decisions
// when scheduling the tasks and will not schedule tasks
// that will fix the drift in other Diff Results.
type LoadBalancersReconciliate struct {
	Hc      *HealthCheckStatus
	Diff    *LoadBalancersDiffResult
	Proxy   *ProxyDiffResult
	Current *spec.ClustersV2
	Desired *spec.ClustersV2
}

// PreKubernetesDiff returns load balancer changes that can be done/executed before
// handling any changes to the kubernetes clusters. Assumes that both the current and
// desired [spec.Clusters] were not modified since the [HealthCheckStatus] and
// [LoadBalancersDiffResult] was computed, and that all of the Cached Indices within
// the [LoadBalancersDiffResult] are not invalidated. This function does not modify the
// input in any way and also the returned [spec.TaskEvent] does not hold or share any
// memory to related to the input.
func PreKubernetesDiff(r LoadBalancersReconciliate) *spec.TaskEventV2 {
	switch r.Diff.ApiEndpoint.State {
	case spec.ApiEndpointChangeStateV2_AttachingLoadBalancerV2:
		// make sure the new lb is already in the cluster.
		if i := clusters.IndexLoadbalancerByIdV2(r.Diff.ApiEndpoint.New, r.Current.LoadBalancers.Clusters); i >= 0 {
			return ScheduleMoveApiEndpoint(r.Current, r.Diff.ApiEndpoint.Current, r.Diff.ApiEndpoint.New, r.Diff.ApiEndpoint.State)
		}
	case spec.ApiEndpointChangeStateV2_DetachingLoadBalancerV2:
		if !r.Hc.Cluster.ControlNodesHave6443 {
			return ScheduleControlNodesPort6443(r.Current, true)
		}
		return ScheduleMoveApiEndpoint(r.Current, r.Diff.ApiEndpoint.Current, r.Diff.ApiEndpoint.New, r.Diff.ApiEndpoint.State)
	case spec.ApiEndpointChangeStateV2_MoveEndpointV2:
		// make sure both are in the current cluster and that the roles have been synced.
		old := clusters.IndexLoadbalancerByIdV2(r.Diff.ApiEndpoint.Current, r.Current.LoadBalancers.Clusters)
		new := clusters.IndexLoadbalancerByIdV2(r.Diff.ApiEndpoint.New, r.Current.LoadBalancers.Clusters)
		oldRolesSynced := len(r.Diff.Modified[r.Diff.ApiEndpoint.Current].Roles.Added) == 0
		newRolesSynced := len(r.Diff.Modified[r.Diff.ApiEndpoint.New].Roles.Added) == 0
		if old >= 0 && new >= 0 && oldRolesSynced && newRolesSynced {
			return ScheduleMoveApiEndpoint(r.Current, r.Diff.ApiEndpoint.Current, r.Diff.ApiEndpoint.New, r.Diff.ApiEndpoint.State)
		}
	case spec.ApiEndpointChangeStateV2_EndpointRenamedV2:
		for lb, modified := range r.Diff.Modified {
			if modified.DNS && lb == r.Diff.ApiEndpoint.Current {
				cid := LoadBalancerIdentifier{
					Id:    lb,
					Index: modified.CurrentIdx,
				}
				did := LoadBalancerIdentifier{
					Id:    lb,
					Index: modified.DesiredIdx,
				}
				return ScheduleReplaceDns(r.Proxy.CurrentUsed, r.Current, r.Desired, cid, did, true)
			}
		}
	case spec.ApiEndpointChangeStateV2_NoChangeV2:
		// Nothing to do.
	}

	// Handle modifications that do not rely on the desired state of
	// the Kubernetes infrastructure to be already existing.
	for lb, modified := range r.Diff.Modified {
		cid := LoadBalancerIdentifier{
			Id:    lb,
			Index: modified.CurrentIdx,
		}
		did := LoadBalancerIdentifier{
			Id:    lb,
			Index: modified.DesiredIdx,
		}

		if modified.DNS {
			return ScheduleReplaceDns(r.Proxy.CurrentUsed, r.Current, r.Desired, cid, did, false)
		}

		if !modified.Dynamic.IsEmpty() {
			return ScheduleReconcileLoadBalancerNodePools(r.Proxy.CurrentUsed, r.Current, r.Desired, cid, did, &modified.Dynamic)
		}

		if !modified.Static.IsEmpty() {
			return ScheduleReconcileLoadBalancerNodePools(r.Proxy.CurrentUsed, r.Current, r.Desired, cid, did, &modified.Static)
		}
	}

	return nil
}

// PostKubernetesDiff returns load balancer changes can be done/executed after handling
// addition/modification changes to the kubernetes clusters. Assumes that both the current
// and desired [spec.Clusters] were not modified since the [HealthCheckStatue] and [LoadBalancersDiffResult]
// was computed, and that all of the Cached Indices within the [LoadBalancersDiffResult]
// are not invalidated. This function does not modify the input in any way and also the
// returned [spec.TaskEvent] does not hold or share any memory to related to the input.
func PostKubernetesDiff(r LoadBalancersReconciliate) *spec.TaskEventV2 {
	for lb, modified := range r.Diff.Modified {
		cid := LoadBalancerIdentifier{
			Id:    lb,
			Index: modified.CurrentIdx,
		}
		did := LoadBalancerIdentifier{
			Id:    lb,
			Index: modified.DesiredIdx,
		}

		// Role modifications need to be handled after changes to the kubernetes cluster
		// namely after halding modifications and additions, as roles from the desired
		// state may reference nodepools that are not yet in the current state which
		// would result in inproper updating of the envoy service on the loadbalancers.
		if len(modified.Roles.Added) > 0 {
			return ScheduleAddRoles(r.Current, r.Desired, cid, did, modified.Roles.Added)
		}
		if len(modified.Roles.Deleted) > 0 {
			return ScheduleDeleteRoles(r.Current, cid, modified.Roles.Deleted)
		}

		// TargetPools modifications needs to be handled after changes to the kubernetes
		// cluster, namely after adding new nodepools as old targetPools could be replaced
		// by new and this could not be handled before the kubernetes changes, as the nodepools
		// would not exist in the cluster yet and would make the workflow break.
		if len(modified.Roles.TargetPoolsAdded) > 0 || len(modified.Roles.TargetPoolsDeleted) > 0 {
			return ScheduleReconcileRoleTargetPools(r.Current, r.Desired, cid, did)
		}
	}

	// Additions must also be handled after additions/modifications to the kubernetes cluster
	// due to the possiblity of having new roles/targetpools that may not yet exist in the
	// current state otherwise.
	for _, lb := range r.Diff.Added {
		return ScheduleJoinLoadBalancer(r.Proxy.CurrentUsed, r.Current, r.Desired, lb)
	}

	// Deletion need to follow after addition.
	for _, lb := range r.Diff.Deleted {
		return ScheduleDeleteLoadBalancer(r.Proxy.CurrentUsed, r.Current, lb)
	}

	if ep := clusters.FindAssignedLbApiEndpointV2(r.Current.LoadBalancers.Clusters); ep != nil {
		if r.Hc.Cluster.ControlNodesHave6443 {
			return ScheduleControlNodesPort6443(r.Current, false)
		}
	}

	return nil
}

// Reconciles the nodepools of the current and desired state based on the provided [NodePoolsDiffResult].
// As nothing special needs to be done with loadbalancer nodepools during addition/deletion of node/nodepools
// therefore the function just takes the nodes/nodepools form the desired state into the current or removes
// them from the current on deletion and returns a [spec.TaskEvent] for the first identified change. The function
// will always prefer to return additions first, until they are fully exhausted, after which deletions are handled.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleReconcileLoadBalancerNodePools(
	useProxy bool,
	current *spec.ClustersV2,
	desired *spec.ClustersV2,
	currentId LoadBalancerIdentifier,
	desiredId LoadBalancerIdentifier,
	diff *NodePoolsDiffResult,
) *spec.TaskEventV2 {
	var (
		desiredLb   = desired.LoadBalancers.Clusters[desiredId.Index]
		toReconcile = proto.Clone(current.LoadBalancers.Clusters[currentId.Index]).(*spec.LBclusterV2)

		inFlight = proto.Clone(current).(*spec.ClustersV2)
		added    = len(diff.PartiallyAdded) > 0 || len(diff.Added) > 0
	)

	for np, nodes := range diff.PartiallyAdded {
		dst := nodepools.FindByName(np, toReconcile.ClusterInfo.NodePools)
		src := nodepools.FindByName(np, desiredLb.ClusterInfo.NodePools)
		nodepools.CopyNodes(dst, src, nodes)
	}

	for np := range diff.Added {
		np := nodepools.FindByName(np, desiredLb.ClusterInfo.NodePools)
		nnp := proto.Clone(np).(*spec.NodePool)
		toReconcile.ClusterInfo.NodePools = append(toReconcile.ClusterInfo.NodePools, nnp)
	}

	if added {
		// Addition Stages
		var (
			tf = spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Reconciling infrastructure for the load balancer",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      "Reconciling firewalls and VMs for new nodes/nodepools",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			}

			ans = spec.Stage_Ansibler{
				Ansibler: &spec.StageAnsibler{
					Description: &spec.StageDescription{
						About:      "Configuring nodes of the reconciled load balancer",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageAnsibler_SubPass{
						{
							Kind: spec.StageAnsibler_INSTALL_VPN,
							Description: &spec.StageDescription{
								About:      "Installing VPN and interconnect new nodes/nodepools with existing infrastructure",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
							Description: &spec.StageDescription{
								About:      "Refreshing/Deploying envoy services for new nodes/nodepools",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			}

			ansProxy = spec.Stage_Ansibler{
				Ansibler: &spec.StageAnsibler{
					Description: &spec.StageDescription{
						About:      "Configuring nodes of the reconciled load balancer",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageAnsibler_SubPass{
						{
							Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
							Description: &spec.StageDescription{
								About:      "Updating HttpProxy,NoProxy environment variables to be used by the package manager",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageAnsibler_INSTALL_VPN,
							Description: &spec.StageDescription{
								About:      "Installing VPN and interconnect new nodes/nodepools with existing infrastructure",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
							Description: &spec.StageDescription{
								About:      "Updating HttpProxy,NoProxy environment variables, after populating private addresses on nodes",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
							Description: &spec.StageDescription{
								About:      "Refreshing/Deploying envoy services for new nodes/nodepools",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageAnsibler_COMMIT_PROXY_ENVS,
							Description: &spec.StageDescription{
								About:      "Commiting proxy environment variables",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			}
		)

		pipeline := []*spec.Stage{
			{StageKind: &tf},
			{StageKind: nil},
		}

		if useProxy {
			pipeline[1].StageKind = &ansProxy
		} else {
			pipeline[1].StageKind = &ans
		}

		updateOp := spec.TaskV2_Update{
			Update: &spec.UpdateV2{
				State: &spec.UpdateV2_State{
					K8S:           inFlight.K8S,
					LoadBalancers: inFlight.LoadBalancers.Clusters,
				},
				Delta: &spec.UpdateV2_TfReconcileLoadBalancer{
					TfReconcileLoadBalancer: &spec.UpdateV2_TerraformerReconcileLoadBalancer{
						Handle: toReconcile,
					},
				},
			},
		}

		return &spec.TaskEventV2{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.EventV2_UPDATE_V2,
			Task:        &spec.TaskV2{Do: &updateOp},
			Description: fmt.Sprintf("Reconciling load balancer %q", currentId.Id),
			Pipeline:    pipeline,
		}
	}

	for np, nodes := range diff.PartiallyDeleted {
		np := nodepools.FindByName(np, toReconcile.ClusterInfo.NodePools)
		nodepools.DeleteNodes(np, nodes)
	}

	for np := range diff.Deleted {
		toReconcile.ClusterInfo.NodePools = nodepools.DeleteByName(toReconcile.ClusterInfo.NodePools, np)
	}

	// Deletion Stages
	var (
		tf = spec.Stage_Terraformer{
			Terraformer: &spec.StageTerraformer{
				Description: &spec.StageDescription{
					About:      "Reconciling infrastructure for the load balancer",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageTerraformer_SubPass{
					{
						Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
						Description: &spec.StageDescription{
							About:      "Remvoing firewalls and nodepools",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}

		ansProxy = spec.Stage_Ansibler{
			Ansibler: &spec.StageAnsibler{
				Description: &spec.StageDescription{
					About:      "Configuring nodes of the reconciled load balancer",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageAnsibler_SubPass{
					{
						Kind: spec.StageAnsibler_INSTALL_VPN,
						Description: &spec.StageDescription{
							About:      "Refreshing VPN on nodes of the cluster after nodes/nodepool deletion",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables, after node/nodepool removal",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_COMMIT_PROXY_ENVS,
						Description: &spec.StageDescription{
							About:      "Commiting proxy environment variables",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}
	)

	// For deletion, the Ansible stage does not need to be executed
	// as there is no need to refresh/reconcile the modified loadbalancer
	// as the deletion deletes the infrastructure and leaves the remaining
	// infrastructure unchanged. Does not affect the roles or the target pools
	// of the loadbalancers in any way.
	//
	// The healthcheck within the reconciliation loop will trigger a refresh
	// of the VPN.
	pipeline := []*spec.Stage{
		{StageKind: &tf},
	}

	// Unless the proxy is in use, in which case the task needs to also
	// update proxy environemnt variables after deletion, in which case
	// the task will also bundle the update of the VPN as there is a call
	// to be made to the Ansibler stage.
	if useProxy {
		next := &spec.Stage{
			StageKind: &ansProxy,
		}
		pipeline = append(pipeline, next)
	}

	updateOp := spec.TaskV2_Update{
		Update: &spec.UpdateV2{
			State: &spec.UpdateV2_State{
				K8S:           inFlight.K8S,
				LoadBalancers: inFlight.LoadBalancers.Clusters,
			},
			Delta: &spec.UpdateV2_TfReconcileLoadBalancer{
				TfReconcileLoadBalancer: &spec.UpdateV2_TerraformerReconcileLoadBalancer{
					Handle: toReconcile,
				},
			},
		},
	}

	return &spec.TaskEventV2{
		Id:          uuid.New().String(),
		Timestamp:   timestamppb.New(time.Now().UTC()),
		Event:       spec.EventV2_UPDATE_V2,
		Task:        &spec.TaskV2{Do: &updateOp},
		Description: fmt.Sprintf("Reconciling load balancer %q", currentId.Id),
		Pipeline:    pipeline,
	}
}

// Replaces the [spec.DNS] in the current state with the [spec.DNS] from the desired state. Based
// on additional provided information via the apiEndpoint boolean, the function will include in
// the scheduled task, steps to interpret the old [spec.DNS] to be the API endpoint and move it
// to the new [spec.DNS.Endpoint].
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleReplaceDns(
	useProxy bool,
	current *spec.ClustersV2,
	desired *spec.ClustersV2,
	cid LoadBalancerIdentifier,
	did LoadBalancerIdentifier,
	apiEndpoint bool,
) *spec.TaskEventV2 {
	var (
		dns       = proto.Clone(desired.LoadBalancers.Clusters[did.Index].Dns).(*spec.DNS)
		inFlight  = proto.Clone(current).(*spec.ClustersV2)
		toReplace = spec.UpdateV2_TerraformerReplaceDns{
			Handle: cid.Id,
			Dns:    dns,
		}
	)

	pipeline := []*spec.Stage{
		{
			StageKind: &spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Reconciling infrastructure",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      "Replacing old DNS infrastructure with new",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		},
	}

	if useProxy {
		pipeline = append(pipeline, &spec.Stage{StageKind: &spec.Stage_Ansibler{
			Ansibler: &spec.StageAnsibler{
				Description: &spec.StageDescription{
					About:      "Configuring nodes after DNS change",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageAnsibler_SubPass{
					{
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_COMMIT_PROXY_ENVS,
						Description: &spec.StageDescription{
							About:      "Commiting proxy environment variables",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}})
	}

	if desired.LoadBalancers.Clusters[did.Index].IsApiEndpoint() && apiEndpoint {
		pst := current.LoadBalancers.Clusters[cid.Index].Dns
		toReplace.OldApiEndpoint = &pst.Endpoint

		pipeline = append(pipeline, &spec.Stage{
			StageKind: &spec.Stage_Ansibler{
				Ansibler: &spec.StageAnsibler{
					Description: &spec.StageDescription{
						About:      "Configuring infrastructure of the cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageAnsibler_SubPass{
						{
							Kind: spec.StageAnsibler_DETERMINE_API_ENDPOINT_CHANGE,
							Description: &spec.StageDescription{
								About:      fmt.Sprintf("Moving API endpoint from %q to the newly configured DNS", pst.Endpoint),
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		})

		pipeline = append(pipeline, &spec.Stage{
			StageKind: &spec.Stage_KubeEleven{
				KubeEleven: &spec.StageKubeEleven{
					Description: &spec.StageDescription{
						About:      "Reconciling kubernetes cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageKubeEleven_SubPass{
						{
							Kind: spec.StageKubeEleven_RECONCILE_CLUSTER,
							Description: &spec.StageDescription{
								About:      "Refreshing kubeconfig after API endpoint change",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		})

		pipeline = append(pipeline, &spec.Stage{
			StageKind: &spec.Stage_Kuber{
				Kuber: &spec.StageKuber{
					Description: &spec.StageDescription{
						About:      "Updating Config Maps after endpoint change",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageKuber_SubPass{
						{
							Kind: spec.StageKuber_PATCH_CLUSTER_INFO_CM,
							Description: &spec.StageDescription{
								About:      "Updating cluster-info cluster map with new endpoint",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageKuber_PATCH_KUBE_PROXY,
							Description: &spec.StageDescription{
								About:      "Updating Kube-Proxy cluster map with new endpoint",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageKuber_PATCH_KUBEADM,
							Description: &spec.StageDescription{
								About:      "Updating Kubeadm cluster map with new endpoint",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
						{
							Kind: spec.StageKuber_CILIUM_RESTART,
							Description: &spec.StageDescription{
								About:      "Performing rollout restart for cilium after changes",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN,
							},
						},
					},
				},
			},
		})
	}

	updateOp := spec.UpdateV2{
		State: &spec.UpdateV2_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.UpdateV2_TfReplaceDns{
			TfReplaceDns: &toReplace,
		},
	}

	task := spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &updateOp,
			},
		},
		Description: fmt.Sprintf("Reconciling DNS for load balancer %q", cid.Id),
		Pipeline:    pipeline,
	}

	return &task
}

// Configures the port 6443 [manifest.APIServerPort] on the control nodes of the [spec.Clusters] state.
// Based on the supplied value of open, the port is either opened or closed on all of the control nodes.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleControlNodesPort6443(current *spec.ClustersV2, open bool) *spec.TaskEventV2 {
	inFlight := proto.Clone(current).(*spec.ClustersV2)
	updateOp := spec.UpdateV2{
		State: &spec.UpdateV2_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.UpdateV2_ClusterApiPort{
			ClusterApiPort: &spec.UpdateV2_ApiPortOnCluster{
				Open: open,
			},
		},
	}

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &updateOp,
			},
		},
		Description: fmt.Sprintf("Reconciling API endpoint port for cluster %q", current.K8S.ClusterInfo.Id()),
		Pipeline: []*spec.Stage{
			{
				StageKind: &spec.Stage_Terraformer{
					Terraformer: &spec.StageTerraformer{
						Description: &spec.StageDescription{
							About:      "Reconciling infrastructure",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageTerraformer_SubPass{
							{
								Kind: spec.StageTerraformer_API_PORT_ON_KUBERNETES,
								Description: &spec.StageDescription{
									About:      "Adjusting firewall rules",
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

// Moves the api endpoint of the kubernetes cluster from the source [spec.Clusters] state to the
// destination [spec.Clusters] state. The API endpoint is moved based on the provided [spec.ApiEndpointChangeState].
// The supplied cid, and did which identify the ID's of the loadbalancers must be valid, if they're not
// the move may fail and yield a broken cluster. This function does not handle moving the api endpoint
// if the kuberentes cluster in [spec.Clusters] does not have Loadbalancers, i.e. only has kubernetes nodes.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleMoveApiEndpoint(
	current *spec.ClustersV2,
	cid string,
	did string,
	change spec.ApiEndpointChangeStateV2,
) *spec.TaskEventV2 {
	inFlight := proto.Clone(current).(*spec.ClustersV2)
	updateOp := spec.UpdateV2{
		State: &spec.UpdateV2_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.UpdateV2_ApiEndpoint_{
			ApiEndpoint: &spec.UpdateV2_ApiEndpoint{
				State:             change,
				CurrentEndpointId: cid,
				DesiredEndpointId: did,
			},
		},
	}

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &updateOp,
			},
		},
		Description: fmt.Sprintf("Moving Api endpoint %s from %q to %q", change.String(), cid, did),
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
								Kind: spec.StageAnsibler_DETERMINE_API_ENDPOINT_CHANGE,
								Description: &spec.StageDescription{
									About:      "Moving api endpoint",
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
							About:      "Reconciling kubernetes cluster",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageKubeEleven_SubPass{
							{
								Kind: spec.StageKubeEleven_RECONCILE_CLUSTER,
								Description: &spec.StageDescription{
									About:      "Refreshing kubeconfig after API endpoint change",
									ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
								},
							},
						},
					},
				},
			},
			{
				StageKind: &spec.Stage_Kuber{
					Kuber: &spec.StageKuber{
						Description: &spec.StageDescription{
							About:      "Updating Config Maps after endpoint change",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageKuber_SubPass{
							{
								Kind: spec.StageKuber_PATCH_CLUSTER_INFO_CM,
								Description: &spec.StageDescription{
									About:      "Updating cluster-info cluster map with new endpoint",
									ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
								},
							},
							{
								Kind: spec.StageKuber_PATCH_KUBE_PROXY,
								Description: &spec.StageDescription{
									About:      "Updating Kube-Proxy cluster map with new endpoint",
									ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
								},
							},
							{
								Kind: spec.StageKuber_PATCH_KUBEADM,
								Description: &spec.StageDescription{
									About:      "Updating Kubeadm cluster map with new endpoint",
									ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
								},
							},
							{
								Kind: spec.StageKuber_CILIUM_RESTART,
								Description: &spec.StageDescription{
									About:      "Performing rollout restart for cilium after changes",
									ErrorLevel: spec.ErrorLevel_ERROR_WARN,
								},
							},
						},
					},
				},
			},
		},
	}
}

// Deletes the loadbalancer with the id specified in the passed in lb from the [spec.Clusters] state.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleDeleteLoadBalancer(useProxy bool, current *spec.ClustersV2, cid LoadBalancerIdentifier) *spec.TaskEventV2 {
	inFlight := proto.Clone(current).(*spec.ClustersV2)
	updateOp := spec.UpdateV2{
		State: &spec.UpdateV2_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.UpdateV2_DeleteLoadBalancer_{
			DeleteLoadBalancer: &spec.UpdateV2_DeleteLoadBalancer{
				Handle: cid.Id,
			},
		},
	}

	// For deletion, the Ansible stage does not need to be executed
	// as there is no need to refresh/reconcile the deleted loadbalancer
	// as the deletion deletes the infrastructure and leaves the remaining
	// infrastructure unchanged. Does not affect the roles or the target pools
	// of the loadbalancers in any way.
	//
	// The healthcheck within the reconciliation loop will trigger a refresh
	// of the VPN.
	pipeline := []*spec.Stage{
		{
			StageKind: &spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Reconciling infrastructure",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      "Destroying infrastructure",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		},
	}

	// Unless the proxy is in use, in which case the task needs to also
	// update proxy environemnt variables after deletion, in which case
	// the task will also bundle the update of the VPN as there is a call
	// to be made to the Ansibler stage.
	if useProxy {
		pipeline = append(pipeline, &spec.Stage{StageKind: &spec.Stage_Ansibler{
			Ansibler: &spec.StageAnsibler{
				Description: &spec.StageDescription{
					About:      "Configuring nodes after Load Balancer deletion",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageAnsibler_SubPass{
					{
						Kind: spec.StageAnsibler_INSTALL_VPN,
						Description: &spec.StageDescription{
							About:      "Refreshing VPN on nodes of the cluster",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_COMMIT_PROXY_ENVS,
						Description: &spec.StageDescription{
							About:      "Commiting proxy environment variables",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}})
	}

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &updateOp,
			},
		},
		Description: fmt.Sprintf("Removing load balancer %q", cid.Id),
		Pipeline:    pipeline,
	}
}

// Joins the loadbalancer with the id specified in the passed in lb from the desired [spec.Clusters] state
// into the existing current infrastructure of [spec.Clusters].
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleJoinLoadBalancer(useProxy bool, current, desired *spec.ClustersV2, did LoadBalancerIdentifier) *spec.TaskEventV2 {
	var (
		toJoin   = proto.Clone(desired.LoadBalancers.Clusters[did.Index]).(*spec.LBclusterV2)
		inFlight = proto.Clone(current).(*spec.ClustersV2)
	)

	// Pipeline stages
	var (
		tf = spec.Stage_Terraformer{
			Terraformer: &spec.StageTerraformer{
				Description: &spec.StageDescription{
					About:      "Creating infrastructure for newly added loadbalancer",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageTerraformer_SubPass{
					{
						Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
						Description: &spec.StageDescription{
							About:      "Spawning infrastructure",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}

		ans = spec.Stage_Ansibler{
			Ansibler: &spec.StageAnsibler{
				Description: &spec.StageDescription{
					About:      "Configuring newly spawned infrastructure",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageAnsibler_SubPass{
					{
						Kind: spec.StageAnsibler_INSTALL_VPN,
						Description: &spec.StageDescription{
							About:      "Installing VPN and interconnect with existing infrastructure",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
						Description: &spec.StageDescription{
							About:      "Setup envoy for roles of the loadbalancer",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}

		ansProxy = spec.Stage_Ansibler{
			Ansibler: &spec.StageAnsibler{
				Description: &spec.StageDescription{
					About:      "Configuring newly spawned infrastructure",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageAnsibler_SubPass{
					{
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables to be used by the package manager",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_INSTALL_VPN,
						Description: &spec.StageDescription{
							About:      "Installing VPN and interconnect with existing infrastructure",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
						Description: &spec.StageDescription{
							About:      "Updating HttpProxy,NoProxy environment variables, after populating private addresses on nodes",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
						Description: &spec.StageDescription{
							About:      "Setup envoy for roles of the loadbalancer",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
					{
						Kind: spec.StageAnsibler_COMMIT_PROXY_ENVS,
						Description: &spec.StageDescription{
							About:      "Commiting proxy environment variables",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}
	)

	updateOp := spec.UpdateV2{
		State: &spec.UpdateV2_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.UpdateV2_TfAddLoadBalancer{
			TfAddLoadBalancer: &spec.UpdateV2_TerraformerAddLoadBalancer{
				Handle: toJoin,
			},
		},
	}

	pipeline := []*spec.Stage{
		{StageKind: &tf},
		{StageKind: nil},
	}

	if useProxy {
		pipeline[1].StageKind = &ansProxy
	} else {
		pipeline[1].StageKind = &ans
	}

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &updateOp,
			},
		},
		Description: fmt.Sprintf("Joining loadbalancer %q into existing infrastructure", did.Id),
		Pipeline:    pipeline,
	}
}

// Removes the passed in roles from the loadbalancer with the id identified from the passed
// in lb string, from the current [spec.Clusters] state.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleDeleteRoles(current *spec.ClustersV2, currentId LoadBalancerIdentifier, roles []string) *spec.TaskEventV2 {
	var (
		toReconcile = proto.Clone(current.LoadBalancers.Clusters[currentId.Index]).(*spec.LBclusterV2)
		inFlight    = proto.Clone(current).(*spec.ClustersV2)
	)

	toReconcile.Roles = slices.DeleteFunc(toReconcile.Roles, func(r *spec.RoleV2) bool {
		return slices.Contains(roles, r.Name)
	})

	updateOp := spec.TaskV2_Update{
		Update: &spec.UpdateV2{
			State: &spec.UpdateV2_State{
				K8S:           inFlight.K8S,
				LoadBalancers: inFlight.LoadBalancers.Clusters,
			},
			Delta: &spec.UpdateV2_TfReconcileLoadBalancer{
				TfReconcileLoadBalancer: &spec.UpdateV2_TerraformerReconcileLoadBalancer{
					Handle: toReconcile,
				},
			},
		},
	}

	return &spec.TaskEventV2{
		Id:          uuid.New().String(),
		Timestamp:   timestamppb.New(time.Now().UTC()),
		Event:       spec.EventV2_UPDATE_V2,
		Task:        &spec.TaskV2{Do: &updateOp},
		Description: fmt.Sprintf("Reconciling load balancer %q", currentId.Id),
		Pipeline: []*spec.Stage{
			{
				StageKind: &spec.Stage_Terraformer{
					Terraformer: &spec.StageTerraformer{
						Description: &spec.StageDescription{
							About:      "Reconciling infrastructure for the load balancer",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageTerraformer_SubPass{
							{
								Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
								Description: &spec.StageDescription{
									About:      "Adjusting firewalls rules after deletetion of roles",
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
							About:      "Configuring nodes of the reconciled load balancer",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageAnsibler_SubPass{
							{
								Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
								Description: &spec.StageDescription{
									About:      "Refreshing existing envoy services and removing unused for deleted roles",
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

// Adds the passed in roles from the loadbalancer with the id identified from the passed
// in lb string, from the desired [spec.Clusters] state into the current [spec.Clusters] state.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleAddRoles(current, desired *spec.ClustersV2, currentId, desiredId LoadBalancerIdentifier, roles []string) *spec.TaskEventV2 {
	var toAdd []*spec.RoleV2
	for _, role := range desired.LoadBalancers.Clusters[desiredId.Index].Roles {
		if slices.Contains(roles, role.Name) {
			toAdd = append(toAdd, proto.Clone(role).(*spec.RoleV2))
		}
	}

	toReconcile := proto.Clone(current.LoadBalancers.Clusters[currentId.Index]).(*spec.LBclusterV2)
	toReconcile.Roles = append(toReconcile.Roles, toAdd...)

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
					Delta: &spec.UpdateV2_TfReconcileLoadBalancer{
						TfReconcileLoadBalancer: &spec.UpdateV2_TerraformerReconcileLoadBalancer{
							Handle: toReconcile,
						},
					},
				},
			},
		},
		Description: fmt.Sprintf("Reconciling load balancer %q", currentId.Id),
		Pipeline: []*spec.Stage{
			{
				StageKind: &spec.Stage_Terraformer{
					Terraformer: &spec.StageTerraformer{
						Description: &spec.StageDescription{
							About:      "Reconciling infrastructure for the load balancer",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageTerraformer_SubPass{
							{
								Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
								Description: &spec.StageDescription{
									About:      "Reconciling firewalls for new roles",
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
							About:      "Configuring nodes of the reconciled load balancer",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageAnsibler_SubPass{
							{
								Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
								Description: &spec.StageDescription{
									About:      "Refreshing existing envoy services and deploy new for newly added roles",
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

// Reconciles the TargetPools in roles from the loadbalancer
// with the id identified from the passed in lb string, from
// the desired [spec.Clusters] state into the current [spec.Clusters]
// state.
//
// The returned [spec.TaskEvent] does not point to or share any
// memory with the two passed in states.
func ScheduleReconcileRoleTargetPools(
	current *spec.ClustersV2,
	desired *spec.ClustersV2,
	currentId LoadBalancerIdentifier,
	desiredId LoadBalancerIdentifier,
) *spec.TaskEventV2 {
	var (
		desiredLb   = desired.LoadBalancers.Clusters[desiredId.Index]
		toReconcile = proto.Clone(current.LoadBalancers.Clusters[currentId.Index]).(*spec.LBclusterV2)
		inFlight    = proto.Clone(current).(*spec.ClustersV2)
	)

	for _, cr := range toReconcile.Roles {
		for _, dr := range desiredLb.Roles {
			if cr.Name == dr.Name {
				cr.TargetPools = slices.Clone(dr.TargetPools)
				break
			}
		}
	}

	// For changing the TargetPools only the envoy services on the LoadBalancer
	// need to be regenerated.
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

					Delta: &spec.UpdateV2_TfReconcileLoadBalancer{
						TfReconcileLoadBalancer: &spec.UpdateV2_TerraformerReconcileLoadBalancer{
							Handle: toReconcile,
						},
					},
				},
			},
		},
		Description: fmt.Sprintf("Reconciling load balancer %q", currentId.Id),
		Pipeline: []*spec.Stage{
			{
				StageKind: &spec.Stage_Ansibler{
					Ansibler: &spec.StageAnsibler{
						Description: &spec.StageDescription{
							About:      "Configuring nodes of the reconciled load balancer",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageAnsibler_SubPass{
							{
								Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
								Description: &spec.StageDescription{
									About:      "Refreshing existing envoy services and deploy new for changed target pools of role",
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
