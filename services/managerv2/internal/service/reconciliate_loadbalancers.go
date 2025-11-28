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

// PreKubernetesDiff returns load balancer changes that can be done/executed before handling any changes to the kubernetes clusters.
// Assumes that both the current and desired [spec.Clusters] were not modified since the [HealthCheckStatue] and [LoadBalancersDiffResult]
// was computed, and that all of the Cached Indices within the [LoadBalancerDiffresult] are not invalidated. This function does not
// modify the input in any way and also the returned [spec.TaskEvent] does not hold or shared any memory to related to the input.
func PreKubernetesDiff(hc *HealthCheckStatus, diff *LoadBalancersDiffResult, current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	for lb, modified := range diff.Modified {
		currentId := LoadBalancerIdentifier{
			Id:    lb,
			Index: modified.CurrentIdx,
		}
		desiredId := LoadBalancerIdentifier{
			Id:    lb,
			Index: modified.DesiredIdx,
		}

		if modified.DNS {
			isApiEndpointChange := diff.ApiEndpoint.State == spec.ApiEndpointChangeStateV2_EndpointRenamedV2
			isApiEndpointChange = isApiEndpointChange && diff.ApiEndpoint.Current.Id == lb
			return replaceDns(current, desired, currentId, desiredId, isApiEndpointChange)
		}

		if len(modified.Roles.Added) > 0 {
			return addRoles(current, desired, currentId, desiredId, modified.Roles.Added)
		}

		if !modified.Dynamic.IsEmpty() {
			return reconcileLoadBalancerNodePools(current, desired, currentId, desiredId, &modified.Dynamic)
		}

		if !modified.Static.IsEmpty() {
			return reconcileLoadBalancerNodePools(current, desired, currentId, desiredId, &modified.Static)
		}
	}

	for _, lb := range diff.Added {
		return joinLoadBalancer(current, desired, lb)
	}

	// TODO move this up the tree as a priority.
	// And check if the loadbalancers are setup before executing.
	// otherwise fallthrough. This should have the highest priority.
	// that way we don't need endless retries.
	switch diff.ApiEndpoint.State {
	case spec.ApiEndpointChangeStateV2_AttachingLoadBalancerV2:
		return moveApiEndpoint(current, diff.ApiEndpoint.Current, diff.ApiEndpoint.New, diff.ApiEndpoint.State)
	case spec.ApiEndpointChangeStateV2_DetachingLoadBalancerV2:
		if !hc.Cluster.ControlNodesHave6443 {
			return controlNodesPort6443(current, true)
		}
		return moveApiEndpoint(current, diff.ApiEndpoint.Current, diff.ApiEndpoint.New, diff.ApiEndpoint.State)
	case spec.ApiEndpointChangeStateV2_MoveEndpointV2:
		return moveApiEndpoint(current, diff.ApiEndpoint.Current, diff.ApiEndpoint.New, diff.ApiEndpoint.State)
	case spec.ApiEndpointChangeStateV2_EndpointRenamedV2:
		// Handled above, with the modified.DNS flag.
	case spec.ApiEndpointChangeStateV2_NoChangeV2:
		// Nothing to do.
	}

	// Deletetion of the roles is placed after changes to the API endpoint,
	// if any, as otherwise removing the roles for the API endpoint would
	// make the cluster broken and unreachable.
	for lb, modified := range diff.Modified {
		if len(modified.Roles.Deleted) > 0 {
			cid := LoadBalancerIdentifier{
				Id:    lb,
				Index: modified.CurrentIdx,
			}
			return deleteRoles(current, cid, modified.Roles.Deleted)
		}
	}

	if ep := clusters.FindAssignedLbApiEndpointV2(current.LoadBalancers.Clusters); ep != nil {
		if hc.Cluster.ControlNodesHave6443 {
			return controlNodesPort6443(current, false)
		}
	}

	return nil
}

// PostKubernetesDiff returns load balancer changes can be done/executed after handling addition/modification changes to the kubernetes clusters.
// Assumes that both the current and desired [spec.Clusters] were not modified since the [HealthCheckStatue] and [LoadBalancersDiffResult]
// was computed, and that all of the Cached Indices within the [LoadBalancerDiffresult] are not invalidated. This function does not
// modify the input in any way and also the returned [spec.TaskEvent] does not hold or shared any memory to related to the input.
func PostKubernetesDiff(_ *HealthCheckStatus, diff *LoadBalancersDiffResult, current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	// TargetPools modifications needs to be handled after changes to the kubernetes
	// cluster, namely after adding new nodepools as old targetPools could be replaced
	// by new and this could not be handled before the kubernetes changes, as the nodepools
	// would not exist in the cluster yet and would make the workflow break.
	for lb, modified := range diff.Modified {
		if len(modified.Roles.TargetPoolsAdded) > 0 || len(modified.Roles.TargetPoolsDeleted) > 0 {
			cid := LoadBalancerIdentifier{
				Id:    lb,
				Index: modified.CurrentIdx,
			}
			did := LoadBalancerIdentifier{
				Id:    lb,
				Index: modified.DesiredIdx,
			}
			return reconcileRoleTargetPools(current, desired, cid, did)
		}
	}

	for _, lb := range diff.Deleted {
		return deleteLoadBalancer(current, lb)
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
func reconcileLoadBalancerNodePools(
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
		updateOp := spec.TaskV2_Update{
			Update: &spec.UpdateV2{
				State: &spec.UpdateV2_State{
					K8S:           inFlight.K8S,
					LoadBalancers: inFlight.LoadBalancers.Clusters,
				},
				Delta: &spec.UpdateV2_ReconcileLoadBalancer_{
					ReconcileLoadBalancer: &spec.UpdateV2_ReconcileLoadBalancer{
						LoadBalancer: toReconcile,
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
										About:      "Reconciling firewalls and nodepools for new nodes/nodepools",
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
					},
				},
			},
		}
	}

	for np, nodes := range diff.PartiallyDeleted {
		np := nodepools.FindByName(np, toReconcile.ClusterInfo.NodePools)
		nodepools.DeleteNodes(np, nodes)
	}

	for np := range diff.Deleted {
		toReconcile.ClusterInfo.NodePools = nodepools.DeleteByName(toReconcile.ClusterInfo.NodePools, np)
	}

	updateOp := spec.TaskV2_Update{
		Update: &spec.UpdateV2{
			State: &spec.UpdateV2_State{
				K8S:           inFlight.K8S,
				LoadBalancers: inFlight.LoadBalancers.Clusters,
			},
			Delta: &spec.UpdateV2_ReconcileLoadBalancer_{
				ReconcileLoadBalancer: &spec.UpdateV2_ReconcileLoadBalancer{
					LoadBalancer: toReconcile,
				},
			},
		},
	}

	// For deletion, the Ansible stage does not need to be executed
	// as there is no need to refresh/reconcile the modified loadbalancer
	// as the deletion deletes the infrastructure and leaves the remaining
	// infrastructure unchanged. For the Wireguard reconciliation the
	// healthcheck will take care of that.
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
									About:      "Remvoing firewalls and nodepools",
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

// Replaces the [spec.DNS] in the current state with the [spec.DNS] from the desired state. Based
// on additional provided information via the apiEndpoint boolean, the function will include in
// the scheduled task, steps to interpret the old [spec.DNS] to be the API endpoint and move it
// to the new [spec.DNS.Endpoint].
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func replaceDns(current, desired *spec.ClustersV2, currentId, desiredId LoadBalancerIdentifier, apiEndpoint bool) *spec.TaskEventV2 {
	var (
		dns      = proto.Clone(desired.LoadBalancers.Clusters[desiredId.Index].Dns).(*spec.DNS)
		inFlight = proto.Clone(current).(*spec.ClustersV2)
	)

	updateOp := spec.UpdateV2{
		State: &spec.UpdateV2_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.UpdateV2_ReplaceDns_{
			ReplaceDns: &spec.UpdateV2_ReplaceDns{
				LoadBalancerId: currentId.Id,
				Dns:            dns,
			},
		},
	}

	// For non API endpoint DNS, it is sufficient only refresh/rebuilt
	// the dns in the Terraformer stage.
	task := spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &updateOp,
			},
		},
		Description: fmt.Sprintf("Reconciling DNS for load balancer %q", currentId.Id),
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
		},
	}

	if desired.LoadBalancers.Clusters[desiredId.Index].IsApiEndpoint() && apiEndpoint {
		pst := current.LoadBalancers.Clusters[currentId.Index].Dns
		updateOp.Delta.(*spec.UpdateV2_ReplaceDns_).ReplaceDns.OldApiEndpoint = &pst.Endpoint

		task.Pipeline = append(task.Pipeline, &spec.Stage{
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

		task.Pipeline = append(task.Pipeline, &spec.Stage{
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
	}

	return &task
}

// Configures the port 6443 [manifest.APIServerPort] on the control nodes of the [spec.Clusters] state.
// Based on the supplied value of open, the port is either opened or closed on all of the control nodes.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func controlNodesPort6443(current *spec.ClustersV2, open bool) *spec.TaskEventV2 {
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
func moveApiEndpoint(
	current *spec.ClustersV2,
	currentId LoadBalancerIdentifier,
	desiredId LoadBalancerIdentifier,
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
				CurrentEndpointId: currentId.Id,
				DesiredEndpointId: desiredId.Id,
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
		Description: fmt.Sprintf("Moving Api endpoint %s from %q to %q", change.String(), currentId.Id, desiredId.Id),
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
		},
	}
}

// Deletes the loadbalancer with the id specified in the passed in lb from the [spec.Clusters] state.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func deleteLoadBalancer(current *spec.ClustersV2, i LoadBalancerIdentifier) *spec.TaskEventV2 {
	inFlight := proto.Clone(current).(*spec.ClustersV2)
	updateOp := spec.UpdateV2{
		State: &spec.UpdateV2_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.UpdateV2_DeleteLoadBalancer_{
			DeleteLoadBalancer: &spec.UpdateV2_DeleteLoadBalancer{
				Id: i.Id,
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
		Description: fmt.Sprintf("Removing load balancer %q", i.Id),
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
		},
	}
}

// Joins the loadbalancer with the id specified in the passed in lb from the desired [spec.Clusters] state
// into the existing current infrastructure of [spec.Clusters].
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func joinLoadBalancer(current, desired *spec.ClustersV2, desiredId LoadBalancerIdentifier) *spec.TaskEventV2 {
	var (
		toJoin   = proto.Clone(desired.LoadBalancers.Clusters[desiredId.Index]).(*spec.LBclusterV2)
		inFlight = proto.Clone(current).(*spec.ClustersV2)
	)

	updateOp := spec.UpdateV2{
		State: &spec.UpdateV2_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.UpdateV2_AddLoadBalancer_{
			AddLoadBalancer: &spec.UpdateV2_AddLoadBalancer{
				LoadBalancer: toJoin,
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
		Description: fmt.Sprintf("Joining loadbalancer %q into existing infrastructure", desiredId.Id),
		Pipeline: []*spec.Stage{
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
									About:      "Spawning infrastructure",
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
				},
			},
		},
	}
}

// Removes the passed in roles from the loadbalancer with the id identified from the passed
// in lb string, from the current [spec.Clusters] state.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func deleteRoles(current *spec.ClustersV2, currentId LoadBalancerIdentifier, roles []string) *spec.TaskEventV2 {
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
			Delta: &spec.UpdateV2_ReconcileLoadBalancer_{
				ReconcileLoadBalancer: &spec.UpdateV2_ReconcileLoadBalancer{
					LoadBalancer: toReconcile,
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
func addRoles(current, desired *spec.ClustersV2, currentId, desiredId LoadBalancerIdentifier, roles []string) *spec.TaskEventV2 {
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
					Delta: &spec.UpdateV2_ReconcileLoadBalancer_{
						ReconcileLoadBalancer: &spec.UpdateV2_ReconcileLoadBalancer{
							LoadBalancer: toReconcile,
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

// Reconciles the TargetPools in roles from the loadbalancer with the id identified from the passed
// in lb string, from the desired [spec.Clusters] state into the current [spec.Clusters] state.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func reconcileRoleTargetPools(current, desired *spec.ClustersV2, currentId, desiredId LoadBalancerIdentifier) *spec.TaskEventV2 {
	var (
		desiredLb   = desired.LoadBalancers.Clusters[desiredId.Index]
		toReconcile = proto.Clone(current.LoadBalancers.Clusters[currentId.Index]).(*spec.LBclusterV2)
		inFlight    = proto.Clone(current).(*spec.ClustersV2)
	)

	for _, cr := range toReconcile.Roles {
		for _, dr := range desiredLb.Roles {
			if cr.Name == dr.Name {
				cr.TargetPools = dr.TargetPools
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
					Delta: &spec.UpdateV2_ReconcileLoadBalancer_{
						ReconcileLoadBalancer: &spec.UpdateV2_ReconcileLoadBalancer{
							LoadBalancer: toReconcile,
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
