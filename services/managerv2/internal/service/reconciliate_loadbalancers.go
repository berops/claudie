package service

import (
	"fmt"
	"slices"
	"time"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func PostKubernetesDiff(hc *HealthCheckStatus, diff *LoadBalancersDiffResult, current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	return nil
}

// PreKubernetesDiff are changes for loadbalancers that can be done/executed before handling changes to the kubernetes clusters.
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
	}

	for _, lb := range diff.Added {
		return joinLoadBalancer(current, desired, lb)
	}

	// TODO: I think modifications to existing lb should have always preference.
	// Then anything else after that should follow.

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

	if ep := clusters.FindAssignedLbApiEndpointV2(current.LoadBalancers.Clusters); ep != nil {
		if hc.Cluster.ControlNodesHave6443 {
			return controlNodesPort6443(current, false)
		}
	}

	// TODO: move this into PostKubernetes Diff.
	// TODO: targetPools deleted. shouldn't that be handled by the modified stage ?
	// for the modified we should create a union that will be created and then after
	// that we simply just delete state that is not in the desired which should be
	// handled by the reconciliation loop ?
	// TODO: handle modified
	for _, lb := range diff.Deleted {
		// TODO: what about the refresh for ansible ?
		return deleteLoadBalancer(current, lb)
	}

	return nil
}

// TODO: with a failed inFlight state there needs to be a decision to be made if a task
// with a higher priority is to be scheduled what happens in that case ?
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
	)

	task := spec.TaskEventV2{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.EventV2_UPDATE_V2,
		State:     inFlight,
		Task: &spec.TaskV2{
			Do: &spec.TaskV2_Update{
				Update: &updateOp,
			},
		},
		Description: "TODO:",
		Pipeline: []*spec.Stage{
			{
				StageKind: &spec.Stage_Terraformer{
					Terraformer: &spec.StageTerraformer{
						Description: &spec.StageDescription{
							About:      "TODO",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageTerraformer_SubPass{
							{
								Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
								Description: &spec.StageDescription{
									About:      "TODO",
									ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
								},
							},
						},
					},
				},
			},
		},
	}

	if desired.LoadBalancers.Clusters[idx].IsApiEndpoint() && apiEndpoint {
		idx := clusters.IndexLoadbalancerByIdV2(lb, current.LoadBalancers.Clusters)
		pst := current.LoadBalancers.Clusters[idx].Dns
		updateOp.Delta.(*spec.UpdateV2_ReplaceDns_).ReplaceDns.OldApiEndpoint = &pst.Endpoint

		task.Pipeline = append(task.Pipeline, &spec.Stage{
			StageKind: &spec.Stage_Ansibler{
				Ansibler: &spec.StageAnsibler{
					Description: &spec.StageDescription{
						About:      "todo",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageAnsibler_SubPass{
						{
							Kind: spec.StageAnsibler_DETERMINE_API_ENDPOINT_CHANGE,
							Description: &spec.StageDescription{
								About:      "TODO",
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
						About:      "TODO",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageKubeEleven_SubPass{
						{
							Kind: spec.StageKubeEleven_RECONCILE_CLUSTER,
							Description: &spec.StageDescription{
								About:      "TODO",
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

	// TODO: remove task options.
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
		Description: "TODO",
		Pipeline: []*spec.Stage{
			{
				StageKind: &spec.Stage_Terraformer{
					Terraformer: &spec.StageTerraformer{
						Description: &spec.StageDescription{
							About:      "Reconciling Api endpoint port",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageTerraformer_SubPass{
							{
								Kind: spec.StageTerraformer_API_PORT_ON_KUBERNETES,
								Description: &spec.StageDescription{
									About:      "Reconciling Api endpoint port",
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
		Description: fmt.Sprintf("Moving Api endpoint %s", change.String()),
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
							About:      "Moving Api endpoint",
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
							About:      "Refresh Kubeconfig after api endpoint move",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageKubeEleven_SubPass{
							{
								Kind: spec.StageKubeEleven_RECONCILE_CLUSTER,
								Description: &spec.StageDescription{
									About:      "refresh kubeconfig after api endpoint move",
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
		Description: "Joining loadbalancer into existing infrastructure",
		// TODO: what about the VPN for the deleted loadbalancers ?
		Pipeline: []*spec.Stage{
			{
				StageKind: &spec.Stage_Terraformer{
					Terraformer: &spec.StageTerraformer{
						Description: &spec.StageDescription{
							About:      "Destroying infrastructure for loadbalancer",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageTerraformer_SubPass{
							{
								Kind: spec.StageTerraformer_DESTROY_INFRASTRUCTURE,
								Description: &spec.StageDescription{
									About:      fmt.Sprintf("TODO: Destroying infrastructure for %q", lb),
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
		Description: "Joining loadbalancer into existing infrastructure",
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

	var (
		idx         = clusters.IndexLoadbalancerByIdV2(lb, current.LoadBalancers.Clusters)
		toReconcile = proto.Clone(current.LoadBalancers.Clusters[idx]).(*spec.LBclusterV2)

		// TODO: maybe we don't need an InFlight state ?
		inFlight = proto.Clone(current).(*spec.ClustersV2)
		state    = proto.Clone(current).(*spec.ClustersV2)
		updateOp = spec.UpdateV2{
			State: &spec.UpdateV2_State{
				K8S:           state.K8S,
				LoadBalancers: state.LoadBalancers.Clusters,
			},
			Delta: &spec.UpdateV2_ReconcileLoadBalancer_{
				ReconcileLoadBalancer: &spec.UpdateV2_ReconcileLoadBalancer{
					LoadBalancer: toReconcile,
				},
			},
		}
	)

	toReconcile.Roles = append(toReconcile.Roles, toAdd...)

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
		Description: "TODO:",
		Pipeline: []*spec.Stage{
			{
				StageKind: &spec.Stage_Terraformer{
					Terraformer: &spec.StageTerraformer{
						Description: &spec.StageDescription{
							About:      "TODO",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageTerraformer_SubPass{
							{
								Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
								Description: &spec.StageDescription{
									About:      "TODO",
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
							About:      "TODO",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageAnsibler_SubPass{
							{
								Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
								Description: &spec.StageDescription{
									About:      "TODO",
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
