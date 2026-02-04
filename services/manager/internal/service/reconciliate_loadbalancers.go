package service

import (
	"fmt"
	"slices"
	"time"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

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
	Current *spec.Clusters
	Desired *spec.Clusters
}

// PreKubernetesDiff returns load balancer changes that can be done/executed before
// handling any changes to the kubernetes clusters. Assumes that both the current and
// desired [spec.Clusters] were not modified since the [HealthCheckStatus] and
// [LoadBalancersDiffResult] was computed, and that all of the Cached Indices within
// the [LoadBalancersDiffResult] are not invalidated. This function does not modify the
// input in any way and also the returned [spec.TaskEvent] does not hold or share any
// memory to related to the input.
func PreKubernetesDiff(r LoadBalancersReconciliate) *spec.TaskEvent {
	switch r.Diff.ApiEndpoint.State {
	case spec.ApiEndpointChangeState_AttachingLoadBalancer:
		// make sure the new lb is already in the cluster.
		if i := clusters.IndexLoadbalancerById(r.Diff.ApiEndpoint.New, r.Current.LoadBalancers.Clusters); i >= 0 {
			return ScheduleMoveApiEndpoint(r.Current, r.Diff.ApiEndpoint.Current, r.Diff.ApiEndpoint.New, r.Diff.ApiEndpoint.State)
		}
	case spec.ApiEndpointChangeState_DetachingLoadBalancer:
		if !r.Hc.Cluster.ControlNodesHave6443 {
			return ScheduleControlNodesPort6443(r.Current, true)
		}
		return ScheduleMoveApiEndpoint(r.Current, r.Diff.ApiEndpoint.Current, r.Diff.ApiEndpoint.New, r.Diff.ApiEndpoint.State)
	case spec.ApiEndpointChangeState_MoveEndpoint:
		// make sure both are in the current cluster and that the roles have been synced.
		old := clusters.IndexLoadbalancerById(r.Diff.ApiEndpoint.Current, r.Current.LoadBalancers.Clusters)
		desired := clusters.IndexLoadbalancerById(r.Diff.ApiEndpoint.New, r.Current.LoadBalancers.Clusters)
		oldRolesSynced := len(r.Diff.Modified[r.Diff.ApiEndpoint.Current].Roles.Added) == 0
		newRolesSynced := len(r.Diff.Modified[r.Diff.ApiEndpoint.New].Roles.Added) == 0
		if old >= 0 && desired >= 0 && oldRolesSynced && newRolesSynced {
			return ScheduleMoveApiEndpoint(r.Current, r.Diff.ApiEndpoint.Current, r.Diff.ApiEndpoint.New, r.Diff.ApiEndpoint.State)
		}
	case spec.ApiEndpointChangeState_EndpointRenamed:
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
	case spec.ApiEndpointChangeState_NoChange:
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

		if len(modified.Dynamic.Added) > 0 || len(modified.Dynamic.PartiallyAdded) > 0 {
			opts := LoadBalancerNodePoolsOptions{
				UseProxy: r.Proxy.CurrentUsed,
				IsStatic: false,
			}
			return ScheduleAdditionLoadBalancerNodePools(r.Current, r.Desired, cid, did, &modified.Dynamic, opts)
		}

		if len(modified.Static.Added) > 0 || len(modified.Static.PartiallyAdded) > 0 {
			opts := LoadBalancerNodePoolsOptions{
				UseProxy: r.Proxy.CurrentUsed,
				IsStatic: true,
			}
			return ScheduleAdditionLoadBalancerNodePools(r.Current, r.Desired, cid, did, &modified.Static, opts)
		}

		if len(modified.Dynamic.Deleted) > 0 || len(modified.Dynamic.PartiallyDeleted) > 0 {
			opts := LoadBalancerNodePoolsOptions{
				UseProxy: r.Proxy.CurrentUsed,
				IsStatic: false,
			}
			return ScheduleDeletionLoadBalancerNodePools(r.Current, cid, &modified.Dynamic, opts)
		}

		if len(modified.Static.Deleted) > 0 || len(modified.Static.PartiallyDeleted) > 0 {
			opts := LoadBalancerNodePoolsOptions{
				UseProxy: r.Proxy.CurrentUsed,
				IsStatic: true,
			}
			return ScheduleDeletionLoadBalancerNodePools(r.Current, cid, &modified.Static, opts)
		}

		if len(modified.PendingDynamicDeletions) > 0 {
			opts := LoadBalancerNodePoolsOptions{
				UseProxy: r.Proxy.CurrentUsed,
				IsStatic: false,
			}

			// Only schedule one node deletion at a time.
			for np, nodes := range modified.PendingDynamicDeletions {
				diff := NodePoolsDiffResult{
					PartiallyDeleted: NodePoolsViewType{
						np: []string{nodes[0]},
					},
				}
				return ScheduleDeletionLoadBalancerNodePools(r.Current, cid, &diff, opts)
			}
		}

		if len(modified.PendingStaticDeletions) > 0 {
			opts := LoadBalancerNodePoolsOptions{
				UseProxy: r.Proxy.CurrentUsed,
				IsStatic: true,
			}

			// Only schedule on node deletion at a time.
			for np, nodes := range modified.PendingStaticDeletions {
				diff := NodePoolsDiffResult{
					PartiallyDeleted: NodePoolsViewType{
						np: []string{nodes[0]},
					},
				}
				return ScheduleDeletionLoadBalancerNodePools(r.Current, cid, &diff, opts)
			}
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
func PostKubernetesDiff(r LoadBalancersReconciliate) *spec.TaskEvent {
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

	if ep := clusters.FindAssignedLbApiEndpoint(r.Current.LoadBalancers.Clusters); ep != nil {
		if r.Hc.Cluster.ControlNodesHave6443 {
			return ScheduleControlNodesPort6443(r.Current, false)
		}
	}

	return nil
}

// LoadBalancersLowPriority handles the very last low priority tasks that should be worked on,
// after all the other changes are done. Assumes that both the current and desired [spec.Clusters]
// were not modified since the [HealthCheckStatus] and [LoadBalancersDiffResult] was computed,
// and that all of the Cached Indices within the [LoadBalancersDiffResult] are not invalidated.
// This function does not modify the input in any way and also the returned [spec.TaskEvent]
// does not hold or share any memory to related to the input.
func LoadBalancersLowPriority(r LoadBalancersReconciliate) *spec.TaskEvent {
	for _, modified := range r.Diff.Modified {
		for np := range modified.RollingUpdate {
			log.Printf("LoadBalaner rolling update: %v", np)
		}
	}

	return nil
}

type LoadBalancerNodePoolsOptions struct {
	UseProxy    bool
	IsStatic    bool
	Unreachable *spec.Unreachable
}

// Schedules a task that will remove nodes/nodepools from the current state of the loadbalancer.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleDeletionLoadBalancerNodePools(
	current *spec.Clusters,
	cid LoadBalancerIdentifier,
	diff *NodePoolsDiffResult,
	opts LoadBalancerNodePoolsOptions,
) *spec.TaskEvent {
	pipeline := []*spec.Stage{}

	if !opts.IsStatic {
		pipeline = append(pipeline, &spec.Stage{
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
								About:      "Remvoing firewalls and nodes",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		})
	}

	// For deletion, the Ansible stage does not need to be executed
	// as there is no need to refresh/reconcile the modified loadbalancer
	// as the deletion deletes the infrastructure and leaves the remaining
	// infrastructure unchanged. Does not affect the roles or the target pools
	// of the loadbalancers in any way.
	//
	// The healthcheck within the reconciliation loop will trigger a refresh
	// of the VPN.
	//
	// Unless the proxy is in use, in which case the task needs to also
	// update proxy environment variables after deletion, in which case
	// the task will also bundle the update of the VPN as there is a call
	// to be made to the Ansibler stage.
	if opts.UseProxy {
		pipeline = append(pipeline, &spec.Stage{
			StageKind: &spec.Stage_Ansibler{
				Ansibler: &spec.StageAnsibler{
					Description: &spec.StageDescription{
						About:      "Configuring nodes of the cluster and the loadbalancers",
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
								About:      "Committing proxy environment variables",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		})
	}

	pipeline = append(pipeline, &spec.Stage{
		StageKind: &spec.Stage_Kuber{
			Kuber: &spec.StageKuber{
				Description: &spec.StageDescription{
					About:      "Configuring kubernetes cluster",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageKuber_SubPass{
					{
						Kind: spec.StageKuber_STORE_LB_SCRAPE_CONFIG,
						Description: &spec.StageDescription{
							About: "Reconciling scrape config",
							// Failing to reconcile scrape config is not a fatal error.
							ErrorLevel: spec.ErrorLevel_ERROR_WARN,
						},
					},
				},
			},
		},
	})

	inFlight := proto.Clone(current).(*spec.Clusters)

	for np, nodes := range diff.PartiallyDeleted {
		update := spec.Task_Update{
			Update: &spec.Update{
				State: &spec.Update_State{
					K8S:           inFlight.K8S,
					LoadBalancers: inFlight.LoadBalancers.Clusters,
				},
				Delta: &spec.Update_None_{},
			},
		}

		if opts.IsStatic {
			// if the nodes to be deleted are static nodes
			// remove them directly from the 'inFlight' state
			// as there is no mechanism within claudie that
			// explicitly removes the static nodes for loadbalanacers
			//
			// With kubernetes nodes the deletion is handled by the
			// kuber service which removes the node, but there isn't
			// such thing for loadbalancers.
			idx := clusters.IndexLoadbalancerById(cid.Id, inFlight.LoadBalancers.Clusters)
			if idx >= 0 {
				lb := inFlight.LoadBalancers.Clusters[idx]
				affectedNodePool := nodepools.FindByName(np, lb.ClusterInfo.NodePools)
				affectedNodes := nodepools.CloneTargetNodes(affectedNodePool, nodes)
				staticNodeKeys := make(map[string]string)

				if stt := affectedNodePool.GetStaticNodePool(); stt != nil {
					for _, n := range affectedNodes {
						key := n.Public
						staticNodeKeys[key] = stt.NodeKeys[key]
					}
				}

				nodepools.DeleteNodes(affectedNodePool, nodes)

				update.Update.Delta = &spec.Update_DeletedLoadBalancerNodes_{
					DeletedLoadBalancerNodes: &spec.Update_DeletedLoadBalancerNodes{
						Unreachable: opts.Unreachable,
						Handle:      cid.Id,
						Kind: &spec.Update_DeletedLoadBalancerNodes_Partial_{
							Partial: &spec.Update_DeletedLoadBalancerNodes_Partial{
								Nodepool:       np,
								Nodes:          affectedNodes,
								StaticNodeKeys: staticNodeKeys,
							},
						},
					},
				}
			}
		} else {
			update.Update.Delta = &spec.Update_TfDeleteLoadBalancerNodes{
				TfDeleteLoadBalancerNodes: &spec.Update_TerraformerDeleteLoadBalancerNodes{
					Unreachable:  opts.Unreachable,
					Handle:       cid.Id,
					WithNodePool: false,
					Nodepool:     np,
					Nodes:        nodes,
				},
			}
		}

		return &spec.TaskEvent{
			Id:        uuid.New().String(),
			Timestamp: timestamppb.New(time.Now().UTC()),
			Event:     spec.Event_UPDATE,
			Task: &spec.Task{
				Do: &update,
			},
			Description: fmt.Sprintf("Deleting %v nodes from nodepool %q of load balancer %q", len(nodes), np, cid.Id),
			Pipeline:    pipeline,
		}
	}

	for np, nodes := range diff.Deleted {
		update := spec.Task_Update{
			Update: &spec.Update{
				State: &spec.Update_State{
					K8S:           inFlight.K8S,
					LoadBalancers: inFlight.LoadBalancers.Clusters,
				},
				Delta: &spec.Update_None_{},
			},
		}

		if opts.IsStatic {
			// Same reason as with partiall deletions.
			idx := clusters.IndexLoadbalancerById(cid.Id, inFlight.LoadBalancers.Clusters)
			if idx >= 0 {
				lb := inFlight.LoadBalancers.Clusters[idx]
				affectedNodePool := nodepools.FindByName(cid.Id, lb.ClusterInfo.NodePools)
				lb.ClusterInfo.NodePools = nodepools.DeleteByName(lb.ClusterInfo.NodePools, np)

				update.Update.Delta = &spec.Update_DeletedLoadBalancerNodes_{
					DeletedLoadBalancerNodes: &spec.Update_DeletedLoadBalancerNodes{
						Unreachable: opts.Unreachable,
						Handle:      cid.Id,
						Kind: &spec.Update_DeletedLoadBalancerNodes_Whole{
							Whole: &spec.Update_DeletedLoadBalancerNodes_WholeNodePool{
								Nodepool: affectedNodePool,
							},
						},
					},
				}
			}
		} else {
			update.Update.Delta = &spec.Update_TfDeleteLoadBalancerNodes{
				TfDeleteLoadBalancerNodes: &spec.Update_TerraformerDeleteLoadBalancerNodes{
					Unreachable:  opts.Unreachable,
					Handle:       cid.Id,
					WithNodePool: true,
					Nodepool:     np,
					Nodes:        nodes,
				},
			}
		}

		return &spec.TaskEvent{
			Id:        uuid.New().String(),
			Timestamp: timestamppb.New(time.Now().UTC()),
			Event:     spec.Event_UPDATE,
			Task: &spec.Task{
				Do: &update,
			},
			Description: fmt.Sprintf("Deleting nodepool %q from load balancer %q", np, cid.Id),
			Pipeline:    pipeline,
		}
	}

	return nil
}

// Schedules a task that will add new nodes/nodepools into the current state of the loadbalancer.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleAdditionLoadBalancerNodePools(
	current *spec.Clusters,
	desired *spec.Clusters,
	cid LoadBalancerIdentifier,
	did LoadBalancerIdentifier,
	diff *NodePoolsDiffResult,
	opts LoadBalancerNodePoolsOptions,
) *spec.TaskEvent {
	pipeline := []*spec.Stage{}

	if !opts.IsStatic {
		pipeline = append(pipeline, &spec.Stage{
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
								About:      "Reconciling firewalls and VMs for new nodes/nodepools",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		})
	}

	ans := spec.Stage_Ansibler{
		Ansibler: &spec.StageAnsibler{
			Description: &spec.StageDescription{
				About:      "Configuring newly added nodes of the load balancer",
				ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
			},
			SubPasses: []*spec.StageAnsibler_SubPass{},
		},
	}

	pipeline = append(pipeline, &spec.Stage{StageKind: &ans})

	if opts.UseProxy {
		ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, []*spec.StageAnsibler_SubPass{
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
				Kind: spec.StageAnsibler_COMMIT_PROXY_ENVS,
				Description: &spec.StageDescription{
					About:      "Committing proxy environment variables",
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
		}...)
	} else {
		ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, []*spec.StageAnsibler_SubPass{
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
		}...)
	}

	pipeline = append(pipeline, &spec.Stage{
		StageKind: &spec.Stage_Kuber{
			Kuber: &spec.StageKuber{
				Description: &spec.StageDescription{
					About:      "Configuring cluster",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageKuber_SubPass{
					{
						Kind: spec.StageKuber_STORE_LB_SCRAPE_CONFIG,
						Description: &spec.StageDescription{
							About: "Reconciling scrape config",
							// Failing to reconcile scrape config is not a fatal error.
							ErrorLevel: spec.ErrorLevel_ERROR_WARN,
						},
					},
				},
			},
		},
	})

	inFlight := proto.Clone(current).(*spec.Clusters)
	for np, nodes := range diff.PartiallyAdded {
		update := spec.Task_Update{
			Update: &spec.Update{
				State: &spec.Update_State{
					K8S:           inFlight.K8S,
					LoadBalancers: inFlight.LoadBalancers.Clusters,
				},
				Delta: nil,
			},
		}

		if opts.IsStatic {
			// For static nodes, merge the nodes directly to the InFlight state,
			// as they do not need to be build, contrary to the dynamic nodes.
			dstlb := inFlight.LoadBalancers.Clusters[cid.Index]
			srclb := desired.LoadBalancers.Clusters[did.Index]

			dst := nodepools.FindByName(np, dstlb.ClusterInfo.NodePools)
			src := nodepools.FindByName(np, srclb.ClusterInfo.NodePools)
			nodepools.CopyNodes(dst, src, nodes)

			update.Update.Delta = &spec.Update_AddedLoadBalancerNodes_{
				AddedLoadBalancerNodes: &spec.Update_AddedLoadBalancerNodes{
					Handle:      cid.Id,
					NewNodePool: false,
					NodePool:    np,
					Nodes:       nodes,
				},
			}
		} else {
			srclb := desired.LoadBalancers.Clusters[did.Index]
			src := nodepools.FindByName(np, srclb.ClusterInfo.NodePools)
			toAdd := nodepools.CloneTargetNodes(src, nodes)

			update.Update.Delta = &spec.Update_TfAddLoadBalancerNodes{
				TfAddLoadBalancerNodes: &spec.Update_TerraformerAddLoadBalancerNodes{
					Handle: cid.Id,
					Kind: &spec.Update_TerraformerAddLoadBalancerNodes_Existing_{
						Existing: &spec.Update_TerraformerAddLoadBalancerNodes_Existing{
							Nodepool: np,
							Nodes:    toAdd,
						},
					},
				},
			}
		}

		return &spec.TaskEvent{
			Id:        uuid.New().String(),
			Timestamp: timestamppb.New(time.Now().UTC()),
			Event:     spec.Event_UPDATE,
			Task: &spec.Task{
				Do: &update,
			},
			Description: fmt.Sprintf("Adding %v nodes to nodepool %q for loadbalancer %q", len(nodes), np, cid.Id),
			Pipeline:    pipeline,
		}
	}

	for np, nodes := range diff.Added {
		srclb := desired.LoadBalancers.Clusters[did.Index]
		src := nodepools.FindByName(np, srclb.ClusterInfo.NodePools)
		toAdd := proto.Clone(src).(*spec.NodePool)

		update := spec.Task_Update{
			Update: &spec.Update{
				State: &spec.Update_State{
					K8S:           inFlight.K8S,
					LoadBalancers: inFlight.LoadBalancers.Clusters,
				},
				Delta: nil,
			},
		}

		if opts.IsStatic {
			// For static nodes, merge directly into the InFlight state,
			// as they do not need to be build, contrary to the dynamic nodes.
			dstlb := inFlight.LoadBalancers.Clusters[cid.Index]
			dstlb.ClusterInfo.NodePools = append(dstlb.ClusterInfo.NodePools, toAdd)

			update.Update.Delta = &spec.Update_AddedLoadBalancerNodes_{
				AddedLoadBalancerNodes: &spec.Update_AddedLoadBalancerNodes{
					Handle:      cid.Id,
					NewNodePool: true,
					NodePool:    np,
					Nodes:       nodes,
				},
			}
		} else {
			update.Update.Delta = &spec.Update_TfAddLoadBalancerNodes{
				TfAddLoadBalancerNodes: &spec.Update_TerraformerAddLoadBalancerNodes{
					Handle: cid.Id,
					Kind: &spec.Update_TerraformerAddLoadBalancerNodes_New_{
						New: &spec.Update_TerraformerAddLoadBalancerNodes_New{
							Nodepool: toAdd,
						},
					},
				},
			}
		}

		return &spec.TaskEvent{
			Id:        uuid.New().String(),
			Timestamp: timestamppb.New(time.Now().UTC()),
			Event:     spec.Event_UPDATE,
			Task: &spec.Task{
				Do: &update,
			},
			Description: fmt.Sprintf("Adding nodepool %q for loadbalancer %q", np, cid.Id),
			Pipeline:    pipeline,
		}
	}

	return nil
}

// Replaces the [spec.DNS] in the current state with the [spec.DNS] from the desired state. Based
// on additional provided information via the apiEndpoint boolean, the function will include in
// the scheduled task, steps to interpret the old [spec.DNS] to be the API endpoint and move it
// to the new [spec.DNS.Endpoint].
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleReplaceDns(
	useProxy bool,
	current *spec.Clusters,
	desired *spec.Clusters,
	cid LoadBalancerIdentifier,
	did LoadBalancerIdentifier,
	apiEndpoint bool,
) *spec.TaskEvent {
	var (
		dns       = proto.Clone(desired.LoadBalancers.Clusters[did.Index].Dns).(*spec.DNS)
		inFlight  = proto.Clone(current).(*spec.Clusters)
		toReplace = spec.Update_TerraformerReplaceDns{
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
							About:      "Committing proxy environment variables",
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

	updateOp := spec.Update{
		State: &spec.Update_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.Update_TfReplaceDns{
			TfReplaceDns: &toReplace,
		},
	}

	task := spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_UPDATE,
		Task: &spec.Task{
			Do: &spec.Task_Update{
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
func ScheduleControlNodesPort6443(current *spec.Clusters, open bool) *spec.TaskEvent {
	inFlight := proto.Clone(current).(*spec.Clusters)
	updateOp := spec.Update{
		State: &spec.Update_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.Update_ClusterApiPort{
			ClusterApiPort: &spec.Update_ApiPortOnCluster{
				Open: open,
			},
		},
	}

	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_UPDATE,
		Task: &spec.Task{
			Do: &spec.Task_Update{
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
	current *spec.Clusters,
	cid string,
	did string,
	change spec.ApiEndpointChangeState,
) *spec.TaskEvent {
	inFlight := proto.Clone(current).(*spec.Clusters)
	updateOp := spec.Update{
		State: &spec.Update_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.Update_ApiEndpoint_{
			ApiEndpoint: &spec.Update_ApiEndpoint{
				State:             change,
				CurrentEndpointId: cid,
				DesiredEndpointId: did,
			},
		},
	}

	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_UPDATE,
		Task: &spec.Task{
			Do: &spec.Task_Update{
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
func ScheduleDeleteLoadBalancer(useProxy bool, current *spec.Clusters, cid LoadBalancerIdentifier) *spec.TaskEvent {
	inFlight := proto.Clone(current).(*spec.Clusters)
	updateOp := spec.Update{
		State: &spec.Update_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.Update_DeleteLoadBalancer_{
			DeleteLoadBalancer: &spec.Update_DeleteLoadBalancer{
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
	// update proxy environment variables after deletion, in which case
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
							About:      "Committing proxy environment variables",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}})
	}

	// If we are deleting the last loadbalancer delete scrape config.
	if len(current.LoadBalancers.Clusters) == 1 {
		pipeline = append(pipeline, &spec.Stage{
			StageKind: &spec.Stage_Kuber{
				Kuber: &spec.StageKuber{
					Description: &spec.StageDescription{
						About:      "Configuring cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageKuber_SubPass{
						{
							Kind: spec.StageKuber_REMOVE_LB_SCRAPE_CONFIG,
							Description: &spec.StageDescription{
								About:      "Removing load balancer scrape config",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN,
							},
						},
					},
				},
			},
		})
	}

	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_UPDATE,
		Task: &spec.Task{
			Do: &spec.Task_Update{
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
func ScheduleJoinLoadBalancer(useProxy bool, current, desired *spec.Clusters, did LoadBalancerIdentifier) *spec.TaskEvent {
	var (
		toJoin   = proto.Clone(desired.LoadBalancers.Clusters[did.Index]).(*spec.LBcluster)
		inFlight = proto.Clone(current).(*spec.Clusters)
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
							About:      "Committing proxy environment variables",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		}

		kuber = spec.Stage_Kuber{
			Kuber: &spec.StageKuber{
				Description: &spec.StageDescription{
					About:      "Configuring cluster",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
				SubPasses: []*spec.StageKuber_SubPass{
					{
						Kind: spec.StageKuber_STORE_LB_SCRAPE_CONFIG,
						Description: &spec.StageDescription{
							About:      "Reconciling load balancer scrape config",
							ErrorLevel: spec.ErrorLevel_ERROR_WARN,
						},
					},
				},
			},
		}
	)

	updateOp := spec.Update{
		State: &spec.Update_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.Update_TfAddLoadBalancer{
			TfAddLoadBalancer: &spec.Update_TerraformerAddLoadBalancer{
				Handle: toJoin,
			},
		},
	}

	pipeline := []*spec.Stage{
		{StageKind: &tf},
		{StageKind: nil},
		{StageKind: &kuber},
	}

	if useProxy {
		pipeline[1].StageKind = &ansProxy
	} else {
		pipeline[1].StageKind = &ans
	}

	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_UPDATE,
		Task: &spec.Task{
			Do: &spec.Task_Update{
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
func ScheduleDeleteRoles(current *spec.Clusters, cid LoadBalancerIdentifier, roles []string) *spec.TaskEvent {
	inFlight := proto.Clone(current).(*spec.Clusters)
	updateOp := spec.Task_Update{
		Update: &spec.Update{
			State: &spec.Update_State{
				K8S:           inFlight.K8S,
				LoadBalancers: inFlight.LoadBalancers.Clusters,
			},
			Delta: &spec.Update_DeleteLoadBalancerRoles_{
				DeleteLoadBalancerRoles: &spec.Update_DeleteLoadBalancerRoles{
					Handle: cid.Id,
					Roles:  roles,
				},
			},
		},
	}

	return &spec.TaskEvent{
		Id:          uuid.New().String(),
		Timestamp:   timestamppb.New(time.Now().UTC()),
		Event:       spec.Event_UPDATE,
		Task:        &spec.Task{Do: &updateOp},
		Description: fmt.Sprintf("Reconciling load balancer %q", cid.Id),
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
			{
				StageKind: &spec.Stage_Kuber{
					Kuber: &spec.StageKuber{
						Description: &spec.StageDescription{
							About:      "Configuring cluster",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageKuber_SubPass{
							{
								Kind: spec.StageKuber_STORE_LB_SCRAPE_CONFIG,
								Description: &spec.StageDescription{
									About:      "Reconciling load balancer scrape config",
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

// Adds the passed in roles from the loadbalancer with the id identified from the passed
// in lb string, from the desired [spec.Clusters] state into the current [spec.Clusters] state.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleAddRoles(current, desired *spec.Clusters, cid, did LoadBalancerIdentifier, roles []string) *spec.TaskEvent {
	var toAdd []*spec.Role
	for _, role := range desired.LoadBalancers.Clusters[did.Index].Roles {
		if slices.Contains(roles, role.Name) {
			toAdd = append(toAdd, proto.Clone(role).(*spec.Role))
		}
	}

	inFlight := proto.Clone(current).(*spec.Clusters)
	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_UPDATE,
		Task: &spec.Task{
			Do: &spec.Task_Update{
				Update: &spec.Update{
					State: &spec.Update_State{
						K8S:           inFlight.K8S,
						LoadBalancers: inFlight.LoadBalancers.Clusters,
					},
					Delta: &spec.Update_TfAddLoadBalancerRoles{
						TfAddLoadBalancerRoles: &spec.Update_TerraformerAddLoadBalancerRoles{
							Handle: cid.Id,
							Roles:  toAdd,
						},
					},
				},
			},
		},
		Description: fmt.Sprintf("Reconciling load balancer %q", cid.Id),
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
			{
				StageKind: &spec.Stage_Kuber{
					Kuber: &spec.StageKuber{
						Description: &spec.StageDescription{
							About:      "Configuring cluster",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageKuber_SubPass{
							{
								Kind: spec.StageKuber_STORE_LB_SCRAPE_CONFIG,
								Description: &spec.StageDescription{
									About:      "Reconciling load balancer scrape config",
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

// Reconciles the TargetPools in roles from the loadbalancer
// with the id identified from the passed in lb string, from
// the desired [spec.Clusters] state into the current [spec.Clusters]
// state.
//
// The returned [spec.TaskEvent] does not point to or share any
// memory with the two passed in states.
func ScheduleReconcileRoleTargetPools(
	current *spec.Clusters,
	desired *spec.Clusters,
	cid LoadBalancerIdentifier,
	did LoadBalancerIdentifier,
) *spec.TaskEvent {
	inFlight := proto.Clone(current).(*spec.Clusters)
	toReconcile := make(map[string]*spec.Update_AnsiblerReplaceTargetPools_TargetPools)

	for _, cr := range inFlight.LoadBalancers.Clusters[cid.Index].Roles {
		for _, dr := range desired.LoadBalancers.Clusters[did.Index].Roles {
			if cr.Name == dr.Name {
				toReconcile[cr.Name] = &spec.Update_AnsiblerReplaceTargetPools_TargetPools{
					Pools: slices.Clone(dr.TargetPools),
				}
				break
			}
		}
	}

	// For changing the TargetPools only the envoy services on the LoadBalancer
	// need to be regenerated.
	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_UPDATE,
		Task: &spec.Task{
			Do: &spec.Task_Update{
				Update: &spec.Update{
					State: &spec.Update_State{
						K8S:           inFlight.K8S,
						LoadBalancers: inFlight.LoadBalancers.Clusters,
					},

					Delta: &spec.Update_AnsReplaceTargetPools{
						AnsReplaceTargetPools: &spec.Update_AnsiblerReplaceTargetPools{
							Handle: cid.Id,
							Roles:  toReconcile,
						},
					},
				},
			},
		},
		Description: fmt.Sprintf("Reconciling load balancer %q", cid.Id),
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
			{
				StageKind: &spec.Stage_Kuber{
					Kuber: &spec.StageKuber{
						Description: &spec.StageDescription{
							About:      "Configuring cluster",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageKuber_SubPass{
							{
								Kind: spec.StageKuber_STORE_LB_SCRAPE_CONFIG,
								Description: &spec.StageDescription{
									About:      "Reconciling load balancer scrape config",
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
