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

// TODO: with a failed inFlight state there needs to be a decision to be made if a task
// with a higher priority is to be scheduled what happens in that case ?
// TODO: maybe restrucurize the workers project directory ?

// PreKubernetesDiff are changes for loadbalancers that can be done/executed before handling any changes to the kubernetes clusters.
func PreKubernetesDiff(hc *HealthCheckStatus, diff *LoadBalancersDiffResult, current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	for lb, modified := range diff.Modified {
		if modified.DNS {
			isApiEndpointChange := diff.ApiEndpoint.State == spec.ApiEndpointChangeStateV2_EndpointRenamedV2
			isApiEndpointChange = isApiEndpointChange && diff.ApiEndpoint.Current == lb
			return replaceDns(current, desired, lb, isApiEndpointChange)
		}

		if len(modified.Roles.Added) > 0 {
			return addRoles(current, desired, lb, modified.Roles.Added)
		}

		if !modified.Dynamic.IsEmpty() {
			return reconcileLoadBalancerNodePools(current, desired, lb, &modified.Dynamic)
		}

		if !modified.Static.IsEmpty() {
			return reconcileLoadBalancerNodePools(current, desired, lb, &modified.Static)
		}
	}

	for _, lb := range diff.Added {
		return joinLoadBalancer(current, desired, lb)
	}

	switch diff.ApiEndpoint.State {
	case spec.ApiEndpointChangeStateV2_AttachingLoadBalancerV2:
		return moveApiEndpoint(current, diff.ApiEndpoint.Current, diff.ApiEndpoint.Desired, diff.ApiEndpoint.State)
	case spec.ApiEndpointChangeStateV2_DetachingLoadBalancerV2:
		if !hc.Cluster.ControlNodesHave6443 {
			return controlNodesPort6443(current, true)
		}
		return moveApiEndpoint(current, diff.ApiEndpoint.Current, diff.ApiEndpoint.Desired, diff.ApiEndpoint.State)
	case spec.ApiEndpointChangeStateV2_MoveEndpointV2:
		return moveApiEndpoint(current, diff.ApiEndpoint.Current, diff.ApiEndpoint.Desired, diff.ApiEndpoint.State)
	case spec.ApiEndpointChangeStateV2_EndpointRenamedV2:
		// Handled above when the DNS is changed.
	case spec.ApiEndpointChangeStateV2_NoChangeV2:
		// Nothing to do.
	}

	for lb, modified := range diff.Modified {
		// Deletetion of the roles is placed after changes
		// to the API endpoint, if any, as otherwise removing
		// the roles for the API endpoint would make the cluster
		// broken and unreachable.
		if len(modified.Roles.Deleted) > 0 {
			return deleteRoles(current, lb, modified.Roles.Deleted)
		}
	}

	if ep := clusters.FindAssignedLbApiEndpointV2(current.LoadBalancers.Clusters); ep != nil {
		if hc.Cluster.ControlNodesHave6443 {
			return controlNodesPort6443(current, false)
		}
	}

	return nil
}

// TODO: check if after each addition the InstallVPN step is called.
// TODO: the reocnciliation could check a random node to see if its wiregurad count matches the count
// of the nodes in the current state if not, refresh.

// PostKubernetesDiff are changes for loadbalancers that can be done/executed after handling changes to the kubernetes clusters.
func PostKubernetesDiff(hc *HealthCheckStatus, diff *LoadBalancersDiffResult, current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	// TargetPools modifications needs to be handler after changes
	// to the kubernetes cluster, namely after adding new nodepools
	// as old targetPools could be replaced by new and this could
	// not be handled before the kubernetes changes, as the nodepools
	// would not exist in the cluster yet and would make the workflow
	// break.
	for lb, modified := range diff.Modified {
		if len(modified.Roles.TargetPoolsAdded) > 0 || len(modified.Roles.TargetPoolsDeleted) > 0 {
			return reconcileRoleTargetPools(current, desired, lb)
		}
	}

	// TODO: what about the refresh for ansible ? -> Healthcheck random node to dump number of wg peers
	// if does not match current state refresh VPN.
	// TODO: what about the VPN for the deleted loadbalancers ? it would need to be refreshed.
	for _, lb := range diff.Deleted {
		return deleteLoadBalancer(current, lb)
	}

	return nil
}

func reconcileLoadBalancerNodePools(
	current *spec.ClustersV2,
	desired *spec.ClustersV2,
	lb string,
	diff *NodePoolsDiffResult,
) *spec.TaskEventV2 {
	var (
		cidx        = clusters.IndexLoadbalancerByIdV2(lb, current.LoadBalancers.Clusters)
		didx        = clusters.IndexLoadbalancerByIdV2(lb, desired.LoadBalancers.Clusters)
		desiredLb   = desired.LoadBalancers.Clusters[didx]
		toReconcile = proto.Clone(current.LoadBalancers.Clusters[cidx]).(*spec.LBclusterV2)
		state       = proto.Clone(current).(*spec.ClustersV2)
		inFlight    = proto.Clone(current).(*spec.ClustersV2)
		added       = len(diff.PartiallyAdded) > 0 || len(diff.Added) > 0
	)

	// nothing special needs to be done with loadbalancer nodepools
	// during addition/deletion of node/nodepools therefore just take
	// the nodes/nodepools form the desired state.

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

	// While there are Additions to be made, make an early return.
	// After all additions are finished the above code will have no
	// side-effects and will fallthrough to the deletion steps.
	if added {
		updateOp := spec.TaskV2_Update{
			Update: &spec.UpdateV2{
				State: &spec.UpdateV2_State{
					K8S:           state.K8S,
					LoadBalancers: state.LoadBalancers.Clusters,
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
			State:       inFlight,
			Task:        &spec.TaskV2{Do: &updateOp},
			Description: fmt.Sprintf("Reconciling load balancer %q", lb),
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
				K8S:           state.K8S,
				LoadBalancers: state.LoadBalancers.Clusters,
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
	// infrastructure unchanged.
	// TODO: what abou VPN for the deleted nodes ?
	return &spec.TaskEventV2{
		Id:          uuid.New().String(),
		Timestamp:   timestamppb.New(time.Now().UTC()),
		Event:       spec.EventV2_UPDATE_V2,
		State:       inFlight,
		Task:        &spec.TaskV2{Do: &updateOp},
		Description: fmt.Sprintf("Reconciling load balancer %q", lb),
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

func replaceDns(current, desired *spec.ClustersV2, lb string, apiEndpoint bool) *spec.TaskEventV2 {
	var (
		idx      = clusters.IndexLoadbalancerByIdV2(lb, desired.LoadBalancers.Clusters)
		dns      = proto.Clone(desired.LoadBalancers.Clusters[idx].Dns).(*spec.DNS)
		inFlight = proto.Clone(current).(*spec.ClustersV2)
		state    = proto.Clone(current).(*spec.ClustersV2)
		updateOp = spec.UpdateV2{
			State: &spec.UpdateV2_State{
				K8S:           state.K8S,
				LoadBalancers: state.LoadBalancers.Clusters,
			},
			Delta: &spec.UpdateV2_ReplaceDns_{
				ReplaceDns: &spec.UpdateV2_ReplaceDns{
					LoadBalancerId: lb,
					Dns:            dns,
				},
			},
		}
		task = spec.TaskEventV2{
			Id:        uuid.New().String(),
			Timestamp: timestamppb.New(time.Now().UTC()),
			Event:     spec.EventV2_UPDATE_V2,
			State:     inFlight,
			Task: &spec.TaskV2{
				Do: &spec.TaskV2_Update{
					Update: &updateOp,
				},
			},
			Description: fmt.Sprintf("Reconciling DNS for load balancer %q", lb),
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
	)

	if desired.LoadBalancers.Clusters[idx].IsApiEndpoint() && apiEndpoint {
		idx := clusters.IndexLoadbalancerByIdV2(lb, current.LoadBalancers.Clusters)
		pst := current.LoadBalancers.Clusters[idx].Dns
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

func controlNodesPort6443(current *spec.ClustersV2, open bool) *spec.TaskEventV2 {
	var (
		inFlight = proto.Clone(current).(*spec.ClustersV2)
		state    = proto.Clone(current).(*spec.ClustersV2)
		updateOp = spec.UpdateV2{
			State: &spec.UpdateV2_State{
				K8S:           state.K8S,
				LoadBalancers: state.LoadBalancers.Clusters,
			},
			Delta: &spec.UpdateV2_ClusterApiPort{
				ClusterApiPort: &spec.UpdateV2_ApiPortOnCluster{
					Open: open,
				},
			},
		}
	)

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

func moveApiEndpoint(current *spec.ClustersV2, cid, did string, change spec.ApiEndpointChangeStateV2) *spec.TaskEventV2 {
	var (
		inFlight = proto.Clone(current).(*spec.ClustersV2)
		state    = proto.Clone(current).(*spec.ClustersV2)
		updateOp = spec.UpdateV2{
			State: &spec.UpdateV2_State{
				K8S:           state.K8S,
				LoadBalancers: state.LoadBalancers.Clusters,
			},
			Delta: &spec.UpdateV2_ApiEndpoint_{
				ApiEndpoint: &spec.UpdateV2_ApiEndpoint{
					State:             change,
					CurrentEndpointId: cid,
					DesiredEndpointId: did,
				},
			},
		}
	)

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
		Description: fmt.Sprintf("Moving Api endpoint %s from %q to %q", change.String(), cid, did),
		OnError: &spec.RetryV2{
			Do: &spec.RetryV2_Repeat_{
				Repeat: &spec.RetryV2_Repeat{
					// TODO: will we need ENdless retries if we will have healthchecking ???
					Kind: spec.RetryV2_Repeat_ENDLESS,
				},
			},
		},
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

func deleteLoadBalancer(current *spec.ClustersV2, lb string) *spec.TaskEventV2 {
	var (
		idx      = clusters.IndexLoadbalancerByIdV2(lb, current.LoadBalancers.Clusters)
		toDelete = proto.Clone(current.LoadBalancers.Clusters[idx]).(*spec.LBclusterV2)
		inFlight = proto.Clone(current).(*spec.ClustersV2)
		deleteOp = spec.DeleteV2{
			Op: &spec.DeleteV2_Loadbalancers{
				Loadbalancers: &spec.DeleteV2_LoadBalancers{
					LoadBalancers: []*spec.LBclusterV2{
						toDelete,
					},
				},
			},
		}
	)

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		State:     inFlight,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Delete{
				Delete: &deleteOp,
			},
		},
		Description: fmt.Sprintf("Removing load balancer %q", lb),
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
								Kind: spec.StageTerraformer_DESTROY_INFRASTRUCTURE,
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
		Description: fmt.Sprintf("Joining loadbalancer %q into existing infrastructure", lb),
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

func deleteRoles(current *spec.ClustersV2, lb string, roles []string) *spec.TaskEventV2 {
	// TODO: maybe we don't need an InFlight state ?
	var (
		idx         = clusters.IndexLoadbalancerByIdV2(lb, current.LoadBalancers.Clusters)
		toReconcile = proto.Clone(current.LoadBalancers.Clusters[idx]).(*spec.LBclusterV2)
		state       = proto.Clone(current).(*spec.ClustersV2)
		inFlight    = proto.Clone(current).(*spec.ClustersV2)
	)

	toReconcile.Roles = slices.DeleteFunc(toReconcile.Roles, func(r *spec.RoleV2) bool {
		return slices.Contains(roles, r.Name)
	})

	updateOp := spec.TaskV2_Update{
		Update: &spec.UpdateV2{
			State: &spec.UpdateV2_State{
				K8S:           state.K8S,
				LoadBalancers: state.LoadBalancers.Clusters,
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
		State:       inFlight,
		Task:        &spec.TaskV2{Do: &updateOp},
		Description: fmt.Sprintf("Reconciling load balancer %q", lb),
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

func addRoles(current, desired *spec.ClustersV2, lb string, roles []string) *spec.TaskEventV2 {
	var toAdd []*spec.RoleV2
	did := clusters.IndexLoadbalancerByIdV2(lb, desired.LoadBalancers.Clusters)
	for _, role := range desired.LoadBalancers.Clusters[did].Roles {
		if slices.Contains(roles, role.Name) {
			toAdd = append(toAdd, proto.Clone(role).(*spec.RoleV2))
		}
	}

	// TODO: maybe we don't need an InFlight state ?
	idx := clusters.IndexLoadbalancerByIdV2(lb, current.LoadBalancers.Clusters)
	toReconcile := proto.Clone(current.LoadBalancers.Clusters[idx]).(*spec.LBclusterV2)
	toReconcile.Roles = append(toReconcile.Roles, toAdd...)

	state := proto.Clone(current).(*spec.ClustersV2)
	inFlight := proto.Clone(current).(*spec.ClustersV2)

	return &spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		State:     inFlight,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &spec.UpdateV2{
					State: &spec.UpdateV2_State{
						K8S:           state.K8S,
						LoadBalancers: state.LoadBalancers.Clusters,
					},
					Delta: &spec.UpdateV2_ReconcileLoadBalancer_{
						ReconcileLoadBalancer: &spec.UpdateV2_ReconcileLoadBalancer{
							LoadBalancer: toReconcile,
						},
					},
				},
			},
		},
		Description: fmt.Sprintf("Reconciling load balancer %q", lb),
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

// TODO: maybe cache the indices instead of querying the index each time ?
func reconcileRoleTargetPools(current, desired *spec.ClustersV2, lb string) *spec.TaskEventV2 {
	var (
		cidx        = clusters.IndexLoadbalancerByIdV2(lb, current.LoadBalancers.Clusters)
		didx        = clusters.IndexLoadbalancerByIdV2(lb, desired.LoadBalancers.Clusters)
		desiredLb   = desired.LoadBalancers.Clusters[didx]
		toReconcile = proto.Clone(current.LoadBalancers.Clusters[cidx]).(*spec.LBclusterV2)
		state       = proto.Clone(current).(*spec.ClustersV2)
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
		State:     inFlight,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &spec.UpdateV2{
					State: &spec.UpdateV2_State{
						K8S:           state.K8S,
						LoadBalancers: state.LoadBalancers.Clusters,
					},
					Delta: &spec.UpdateV2_ReconcileLoadBalancer_{
						ReconcileLoadBalancer: &spec.UpdateV2_ReconcileLoadBalancer{
							LoadBalancer: toReconcile,
						},
					},
				},
			},
		},
		Description: fmt.Sprintf("Reconciling load balancer %q", lb),
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
