package service

import (
	"fmt"
	"time"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Wraps data and diffs needed by the reconciliation
// for kubernetes cluster.
//
// The Kubernetes reconciliation will only consider
// fixing drifts in the passed [KubernetesDiffResult].
//
// Others will only be used for guiding the decisions
// when scheduling the tasks and will not schedule tasks
// that will fix the drift in other Diff Results.
type KubernetesReconciliate struct {
	Hc      *HealthCheckStatus
	Diff    *KubernetesDiffResult
	Current *spec.Clusters
	Desired *spec.Clusters
}

// KubernetesModifications returns kubernetes cluster changes that can be done/executed before
// handling any deletion changes of clusters either, kubernetes of loadbalancers. Assumes that
// both the current and desired [spec.Clusters] were not modified since the [HealthCheckStatus]
// and [KubernetesDiffResult] was computed, and that all of the Cached Indices within the
// [KubernetesDiffResult] are not invalidated. This function does not modify the input in any
// way and also the returned [spec.TaskEvent] does not hold or share any memory to related to the input.
func KubernetesModifications(r KubernetesReconciliate) *spec.TaskEvent {
	if r.Diff.ApiEndpoint.Current != "" && r.Diff.ApiEndpoint.Desired != "" {
		// make sure the desired node is already in the current state.
		transfer := r.Diff.ApiEndpoint.Current != r.Diff.ApiEndpoint.Desired
		transfer = transfer && nodepools.ContainsNode(r.Current.K8S.ClusterInfo.NodePools, r.Diff.ApiEndpoint.Desired)
		if transfer {
			return ScheduleTransferApiEndpoint(r.Current, r.Diff.ApiEndpoint.DesiredNodePool, r.Diff.ApiEndpoint.Desired)
		}
	}

	if len(r.Diff.Dynamic.Added) > 0 || len(r.Diff.Dynamic.PartiallyAdded) > 0 {
		opts := K8sNodeAdditionOptions{
			UseProxy:     r.Diff.Proxy.CurrentUsed,
			HasApiServer: r.Diff.ApiEndpoint.Current != "",
			IsStatic:     false,
		}
		return ScheduleAdditionsInNodePools(r.Current, r.Desired, &r.Diff.Dynamic, opts)
	}

	if len(r.Diff.Static.Added) > 0 || len(r.Diff.Static.PartiallyAdded) > 0 {
		opts := K8sNodeAdditionOptions{
			UseProxy:     r.Diff.Proxy.CurrentUsed,
			HasApiServer: r.Diff.ApiEndpoint.Current != "",
			IsStatic:     true,
		}
		return ScheduleAdditionsInNodePools(r.Current, r.Desired, &r.Diff.Static, opts)
	}

	return nil
}

// KubernetesDeletions returns kubernetes cluster changes can be done/executed after handling
// addition/modification changes to the kubernetes clusters, and deletions of loadbalancers.
// Assumes that both the current and desired [spec.Clusters] were not modified since the
// [HealthCheckStatus] and [KubernetesDiffResult] was computed, and that all of the Cached
// Indices within the [KubernetesDiffResult] are not invalidated. This function does not modify
// the input in any way and also the returned [spec.TaskEvent] does not hold or share any
// memory to related to the input.
func KubernetesDeletions(r KubernetesReconciliate) *spec.TaskEvent {
	if len(r.Diff.Dynamic.Deleted) > 0 || len(r.Diff.Dynamic.PartiallyDeleted) > 0 {
		opts := K8sNodeDeletionOptions{
			UseProxy:     r.Diff.Proxy.CurrentUsed,
			HasApiServer: r.Diff.ApiEndpoint.Current != "",
			IsStatic:     false,
		}
		return ScheduleDeletionsInNodePools(r.Current, &r.Diff.Dynamic, opts)
	}

	if len(r.Diff.Static.Deleted) > 0 || len(r.Diff.Static.PartiallyDeleted) > 0 {
		opts := K8sNodeDeletionOptions{
			UseProxy:     r.Diff.Proxy.CurrentUsed,
			HasApiServer: r.Diff.ApiEndpoint.Current != "",
			IsStatic:     true,
		}
		return ScheduleDeletionsInNodePools(r.Current, &r.Diff.Static, opts)
	}

	if len(r.Diff.PendingDynamicDeletions) > 0 {
		opts := K8sNodeDeletionOptions{
			UseProxy:     r.Diff.Proxy.CurrentUsed,
			HasApiServer: r.Diff.ApiEndpoint.Current != "",
			IsStatic:     false,
		}

		// Only schedule one node deletion at a time.
		for np, nodes := range r.Diff.PendingDynamicDeletions {
			diff := NodePoolsDiffResult{
				PartiallyDeleted: NodePoolsViewType{
					np: []string{nodes[0]},
				},
			}
			return ScheduleDeletionsInNodePools(r.Current, &diff, opts)
		}
	}

	if len(r.Diff.PendingStaticDeletions) > 0 {
		opts := K8sNodeDeletionOptions{
			UseProxy:     r.Diff.Proxy.CurrentUsed,
			HasApiServer: r.Diff.ApiEndpoint.Current != "",
			IsStatic:     true,
		}

		// Only schedule one node deletion at a time.
		for np, nodes := range r.Diff.PendingStaticDeletions {
			diff := NodePoolsDiffResult{
				PartiallyDeleted: NodePoolsViewType{
					np: []string{nodes[0]},
				},
			}
			return ScheduleDeletionsInNodePools(r.Current, &diff, opts)
		}
	}

	return nil
}

// KubernetesLowPriority handles the very last low priority tasks that should be worked on,
// after all the other changes are done. Assumes that both the current and desired [spec.Clusters]
// were not modified since the [HealthCheckStatus] and [KubernetesDiffResult] was computed,
// and that all of the Cached Indices within the [KubernetesDiffResult] are not invalidated.
// This function does not modify the input in any way and also the returned [spec.TaskEvent]
// does not hold or share any memory to related to the input.
func KubernetesLowPriority(r KubernetesReconciliate) *spec.TaskEvent {
	if r.Diff.Version {
		return ScheduleUpgradeKubernetesVersion(r.Current, r.Desired)
	}

	switch r.Diff.Proxy.Change {
	case ProxyOff:
		return ScheduleProxyOff(r.Current, r.Desired)
	case ProxyOn:
		return ScheduleProxyOn(r.Current, r.Desired)
	}

	labels := len(r.Diff.LabelsTaintsAnnotations.Added.Labels) > 0
	labels = labels || len(r.Diff.LabelsTaintsAnnotations.Deleted.LabelKeys) > 0

	annotations := len(r.Diff.LabelsTaintsAnnotations.Added.Annotations) > 0
	annotations = annotations || len(r.Diff.LabelsTaintsAnnotations.Deleted.AnnotationsKeys) > 0

	taints := len(r.Diff.LabelsTaintsAnnotations.Added.Taints) > 0
	taints = taints || len(r.Diff.LabelsTaintsAnnotations.Deleted.TaintKeys) > 0

	if labels || annotations || taints {
		return SchedulePatchNodes(r.Current, r.Diff.LabelsTaintsAnnotations)
	}

	return nil
}

// Schedules a task that will update the kubernetes version to the new desired version of the cluster.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleUpgradeKubernetesVersion(current, desired *spec.Clusters) *spec.TaskEvent {
	// TODO: this cannot be rollbacked once the rollback mechanism is implemented.
	// same way deletion shouldn't be rolled back on error. Same way The Change
	// of the Api Endpoint shouldn't be rolled back, those should be tried
	// infinitely or just ignore errors as some point ?

	inFlight := proto.Clone(current).(*spec.Clusters)
	toReplace := desired.K8S.Kubernetes

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
					Delta: &spec.Update_UpgradeVersion_{
						UpgradeVersion: &spec.Update_UpgradeVersion{
							Version: toReplace,
						},
					},
				},
			},
		},
		Description: "Updating kubernetes version",
		Pipeline: []*spec.Stage{
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
									About:      "Rolling update of the nodes",
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

// Schedules a task that will turn off the HttpProxy and NoProxy env variables across the nodes
// of the cluster.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleProxyOff(current, desired *spec.Clusters) *spec.TaskEvent {
	toReplace := proto.Clone(desired.K8S.InstallationProxy).(*spec.InstallationProxy)
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
					Delta: &spec.Update_AnsReplaceProxy{
						AnsReplaceProxy: &spec.Update_AnsiblerReplaceProxySettings{
							Proxy: toReplace,
						},
					},
				},
			},
		},
		Description: "Refreshing Proxy Environment",
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
								Kind: spec.StageAnsibler_CLEAR_PROXY_ENVS_ON_NODES,
								Description: &spec.StageDescription{
									About:      "Clearing Proxy environment variables across nodes of the cluster",
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
			},
		},
	}
}

// Schedules a task that will turn on the HttpProxy and NoProxy env variables across the nodes
// of the cluster.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleProxyOn(current, desired *spec.Clusters) *spec.TaskEvent {
	toReplace := proto.Clone(desired.K8S.InstallationProxy).(*spec.InstallationProxy)
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
					Delta: &spec.Update_AnsReplaceProxy{
						AnsReplaceProxy: &spec.Update_AnsiblerReplaceProxySettings{
							Proxy: toReplace,
						},
					},
				},
			},
		},
		Description: "Refreshing Proxy Environment",
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
								Kind: spec.StageAnsibler_UPDATE_PROXY_ENVS_ON_NODES,
								Description: &spec.StageDescription{
									About:      "Updating Proxy environment variables across nodes of the cluster",
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
			},
		},
	}
}

// Schedules a task that will re-patch the nodes with the new `taints`,`annotations`,`labels`
// of the cluster.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func SchedulePatchNodes(current *spec.Clusters, diff LabelsTaintsAnnotationsDiffResult) *spec.TaskEvent {
	var (
		inFlight = proto.Clone(current).(*spec.Clusters)
		toPatch  = spec.Update_KuberPatchNodes{
			Add: &spec.Update_KuberPatchNodes_AddBatch{
				Taints:      make(map[string]*spec.Update_KuberPatchNodes_ListOfTaints),
				Labels:      make(map[string]*spec.Update_KuberPatchNodes_MapOfLabels),
				Annotations: make(map[string]*spec.Update_KuberPatchNodes_MapOfAnnotations),
			},
			Remove: &spec.Update_KuberPatchNodes_RemoveBatch{
				Taints:      make(map[string]*spec.Update_KuberPatchNodes_ListOfTaints),
				Annotations: make(map[string]*spec.Update_KuberPatchNodes_ListOfAnnotationKeys),
				Labels:      make(map[string]*spec.Update_KuberPatchNodes_ListOfLabelKeys),
			},
		}
	)

	for np, labels := range diff.Added.Labels {
		toPatch.Add.Labels[np] = &spec.Update_KuberPatchNodes_MapOfLabels{
			Labels: labels,
		}
	}

	for np, annotations := range diff.Added.Annotations {
		toPatch.Add.Annotations[np] = &spec.Update_KuberPatchNodes_MapOfAnnotations{
			Annotations: annotations,
		}
	}

	for np, taints := range diff.Added.Taints {
		toPatch.Add.Taints[np] = &spec.Update_KuberPatchNodes_ListOfTaints{
			Taints: taints,
		}
	}

	for np, keys := range diff.Deleted.LabelKeys {
		toPatch.Remove.Labels[np] = &spec.Update_KuberPatchNodes_ListOfLabelKeys{
			Labels: keys,
		}
	}

	for np, keys := range diff.Deleted.AnnotationsKeys {
		toPatch.Remove.Annotations[np] = &spec.Update_KuberPatchNodes_ListOfAnnotationKeys{
			Annotations: keys,
		}
	}

	for np, taints := range diff.Deleted.TaintKeys {
		toPatch.Remove.Taints[np] = &spec.Update_KuberPatchNodes_ListOfTaints{
			Taints: taints,
		}
	}

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
					Delta: &spec.Update_KpatchNodes{
						KpatchNodes: &toPatch,
					},
				},
			},
		},
		Description: "Patching kubernetes nodes",
		Pipeline: []*spec.Stage{
			{
				StageKind: &spec.Stage_Kuber{
					Kuber: &spec.StageKuber{
						Description: &spec.StageDescription{
							About:      "Reconciling changes to Taints/Annotaions/Labels",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
						SubPasses: []*spec.StageKuber_SubPass{
							{
								Kind: spec.StageKuber_PATCH_NODES,
								Description: &spec.StageDescription{
									About:      "Reconciling changes",
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

// Schedules a task that transfer the Api endpoint from the current node of the kubernetes
// cluster to the new desired node within the cluster
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleTransferApiEndpoint(current *spec.Clusters, nodepool, node string) *spec.TaskEvent {
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
					Delta: &spec.Update_K8SApiEndpoint{
						K8SApiEndpoint: &spec.Update_K8SOnlyApiEndpoint{
							Nodepool: nodepool,
							Node:     node,
						},
					},
				},
			},
		},
		Description: fmt.Sprintf("Transferring Api endpoint to %s from nodepool %s", node, nodepool),
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
								Kind: spec.StageAnsibler_UPDATE_API_ENDPOINT,
								Description: &spec.StageDescription{
									About:      "Transferring api endpoint",
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

type K8sNodeAdditionOptions struct {
	UseProxy     bool
	HasApiServer bool
	IsStatic     bool
}

// Schedules a task that will add new nodes/nodepools into the current state of the cluster.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleAdditionsInNodePools(
	current *spec.Clusters,
	desired *spec.Clusters,
	diff *NodePoolsDiffResult,
	opts K8sNodeAdditionOptions,
) *spec.TaskEvent {
	inFlight := proto.Clone(current).(*spec.Clusters)
	pipeline := []*spec.Stage{}

	if !opts.IsStatic {
		pipeline = append(pipeline, &spec.Stage{
			StageKind: &spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Reconciling infrastructure for kubernetes cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      "Spawning VMs for new nodes",
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
				About:      "Configuring nodes of the cluster and its loadbalancers",
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
				Kind: spec.StageAnsibler_INSTALL_NODE_REQUIREMENTS,
				Description: &spec.StageDescription{
					About:      "Installing node requirments for newly added nodes",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
			},
			{
				Kind: spec.StageAnsibler_INSTALL_TEE_OVERRIDE,
				Description: &spec.StageDescription{
					About:      "Installing Tee override for newly added nodes",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
			},
			{
				Kind: spec.StageAnsibler_INSTALL_VPN,
				Description: &spec.StageDescription{
					About:      "Installing VPN and interconnect new nodes with existing infrastructure",
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
		}...)
	} else {
		ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, []*spec.StageAnsibler_SubPass{
			{
				Kind: spec.StageAnsibler_INSTALL_NODE_REQUIREMENTS,
				Description: &spec.StageDescription{
					About:      "Installing node requirments for newly added nodes",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
			},
			{
				Kind: spec.StageAnsibler_INSTALL_TEE_OVERRIDE,
				Description: &spec.StageDescription{
					About:      "Installing Tee override for newly added nodes",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
			},
			{
				Kind: spec.StageAnsibler_INSTALL_VPN,
				Description: &spec.StageDescription{
					About:      "Installing VPN and interconnect new nodes with existing infrastructure",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
			},
		}...)
	}

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
							About:      "Joining new nodes into the kubernetes cluster",
							ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
						},
					},
				},
			},
		},
	})

	kuber := spec.Stage_Kuber{
		Kuber: &spec.StageKuber{
			Description: &spec.StageDescription{
				About:      "Configuring kubernetes cluster",
				ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
			},
			SubPasses: []*spec.StageKuber_SubPass{
				{
					Kind: spec.StageKuber_DEPLOY_KUBELET_CSR_APPROVER,
					Description: &spec.StageDescription{
						About: "Deploying kubelet-csr approver",
						// Failing to deploy the approve is not a fatal error
						// that would make the whole cluster unusable.
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
				},
				{
					Kind: spec.StageKuber_PATCH_NODES,
					Description: &spec.StageDescription{
						About: "Patching newly added nodes",
						// Failing to patch  nodes is not a fatal error.
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
				},
			},
		},
	}

	pipeline = append(pipeline, &spec.Stage{StageKind: &kuber})

	for np, nodes := range diff.PartiallyAdded {
		// If the ApiServer is on the kubernetes cluster on addition of the
		// control plane nodes the Kubeadm config map needs to be updated which
		// is used during the join operation of new nodes.
		if opts.HasApiServer && nodepools.FindByName(np, current.K8S.ClusterInfo.NodePools).IsControl {
			kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, []*spec.StageKuber_SubPass{
				{
					Kind: spec.StageKuber_PATCH_KUBEADM,
					Description: &spec.StageDescription{
						About:      "Updating Kubeadm certSANs",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
				},
				{
					Kind: spec.StageKuber_CILIUM_RESTART,
					Description: &spec.StageDescription{
						About: "Rollout restart of cilium pods",
						// Rollout restart failing is not a fatal error.
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
				},
			}...)
		}

		// If changes to the nodepool affects any loadbalancer
		// also schedule a reconciliation of loadbalancers, as
		// the envoy targets needs to be regenerated.
		for _, lb := range current.LoadBalancers.Clusters {
			for _, r := range lb.Roles {
				for _, tg := range r.TargetPools {
					// Need to match against only the nodepool name without the hash.
					if n, _ := nodepools.MatchNameAndHashWithTemplate(tg, np); n != "" {
						ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, &spec.StageAnsibler_SubPass{
							Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
							Description: &spec.StageDescription{
								About:      "Reconciling Envoy settings after changes to nodepool",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						})
					}
				}
			}
		}

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
			// For static nodes, merge the nodes already into the inFlight state,
			// as they do not need to be build, contrary to dynamic nodes.
			dst := nodepools.FindByName(np, inFlight.K8S.ClusterInfo.NodePools)
			src := nodepools.FindByName(np, desired.K8S.ClusterInfo.NodePools)
			nodepools.CopyNodes(dst, src, nodes)
			update.Update.Delta = &spec.Update_AddedK8SNodes_{
				AddedK8SNodes: &spec.Update_AddedK8SNodes{
					Nodepool:    np,
					Nodes:       nodes,
					NewNodePool: false,
				},
			}
		} else {
			src := nodepools.FindByName(np, desired.K8S.ClusterInfo.NodePools)
			toAdd := nodepools.CloneTargetNodes(src, nodes)
			update.Update.Delta = &spec.Update_TfAddK8SNodes{
				TfAddK8SNodes: &spec.Update_TerraformerAddK8SNodes{
					Kind: &spec.Update_TerraformerAddK8SNodes_Existing_{
						Existing: &spec.Update_TerraformerAddK8SNodes_Existing{
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
			Description: fmt.Sprintf("Adding %v nodes into nodepool %s", len(nodes), np),
			Pipeline:    pipeline,
		}
	}

	for np, nodes := range diff.Added {
		toAdd := nodepools.FindByName(np, desired.K8S.ClusterInfo.NodePools)
		toAdd = proto.Clone(toAdd).(*spec.NodePool)

		// If the ApiServer is on the kubernetes cluster on addition of the
		// control plane nodes the Kubeadm config map needs to be updated which
		// is used during the join operation of new nodes.
		if opts.HasApiServer && toAdd.IsControl {
			kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, []*spec.StageKuber_SubPass{
				{
					Kind: spec.StageKuber_PATCH_KUBEADM,
					Description: &spec.StageDescription{
						About:      "Updating Kubeadm certSANs",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
				},
				{
					Kind: spec.StageKuber_CILIUM_RESTART,
					Description: &spec.StageDescription{
						About: "Rollout restart of cilium pods",
						// Rollout restart failing is not a fatal error.
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
				},
			}...)
		}

		enableCA := len(nodepools.Autoscaled(current.K8S.ClusterInfo.NodePools)) == 0
		enableCA = enableCA && nodepools.IsAutoscaled(toAdd)
		if enableCA {
			kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, &spec.StageKuber_SubPass{
				Kind: spec.StageKuber_ENABLE_LONGHORN_CA,
				Description: &spec.StageDescription{
					About:      "Enable cluster-autoscaler support for longhorn",
					ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
				},
			})
		}

		// On Addition of a new nodepool, reconcile the storage classes for longhorn.
		kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, &spec.StageKuber_SubPass{
			Kind: spec.StageKuber_RECONCILE_LONGHORN_STORAGE_CLASSES,
			Description: &spec.StageDescription{
				About:      "Reconciling claudie longhorn storage classes after new nodepool",
				ErrorLevel: spec.ErrorLevel_ERROR_WARN,
			},
		})

		// If changes to the nodepool affects any loadbalancer
		// also schedule a reconciliation of loadbalancers, as
		// the envoy targets needs to be regenerated.
		for _, lb := range current.LoadBalancers.Clusters {
			for _, r := range lb.Roles {
				for _, tg := range r.TargetPools {
					// Need to match against only the nodepool name without the hash.
					if n, _ := nodepools.MatchNameAndHashWithTemplate(tg, np); n != "" {
						ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, &spec.StageAnsibler_SubPass{
							Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
							Description: &spec.StageDescription{
								About:      "Reconciling Envoy settings after changes to nodepool",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						})
					}
				}
			}
		}

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
			// For static nodes, merge the nodes already into the inFlight state,
			// as they do not need to be build, contrary to dynamic nodes.
			inFlight.K8S.ClusterInfo.NodePools = append(inFlight.K8S.ClusterInfo.NodePools, toAdd)
			update.Update.Delta = &spec.Update_AddedK8SNodes_{
				AddedK8SNodes: &spec.Update_AddedK8SNodes{
					NewNodePool: true,
					Nodepool:    np,
					Nodes:       nodes,
				},
			}
		} else {
			update.Update.Delta = &spec.Update_TfAddK8SNodes{
				TfAddK8SNodes: &spec.Update_TerraformerAddK8SNodes{
					Kind: &spec.Update_TerraformerAddK8SNodes_New_{
						New: &spec.Update_TerraformerAddK8SNodes_New{
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
			Description: fmt.Sprintf("Adding nodepool %s", np),
			Pipeline:    pipeline,
		}
	}

	return nil
}

type K8sNodeDeletionOptions struct {
	UseProxy     bool
	HasApiServer bool
	IsStatic     bool

	// Optional unreachable infrastructure that will
	// be passed along the scheduled deletion [spec.TaskEvent]
	Unreachable *spec.Unreachable
}

// Schedules a task that will delete nodes/nodepools from the current state of the cluster.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleDeletionsInNodePools(
	current *spec.Clusters,
	diff *NodePoolsDiffResult,
	opts K8sNodeDeletionOptions,
) *spec.TaskEvent {
	inFlight := proto.Clone(current).(*spec.Clusters)

	kuber := spec.Stage_Kuber{
		Kuber: &spec.StageKuber{
			Description: &spec.StageDescription{
				About:      "Reconciling cluster configuration",
				ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
			},
			SubPasses: []*spec.StageKuber_SubPass{
				{
					Kind: spec.StageKuber_DELETE_NODES,
					Description: &spec.StageDescription{
						About:      "Deleting nodes from kubernetes cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
				},
			},
		},
	}

	// For deletion, the Ansible stage does not need to be executed
	// as there is no need to refresh/reconcile the modified nodepools
	// as the deletion deletes the infrastructure and leaves the remaining
	// infrastructure unchanged. Does not affect the cluster or the loadbalancers
	// in any way.
	//
	// The healthcheck within the reconciliation loop will trigger a refresh
	// of the VPN, which is the only action to do on deletion, but leaving it
	// to the healthcheck may buffer more nodes in a single update.
	pipeline := []*spec.Stage{
		{StageKind: &kuber},
	}

	ans := spec.Stage_Ansibler{
		Ansibler: &spec.StageAnsibler{
			Description: &spec.StageDescription{
				About:      "Configuring nodes of the cluster and its loadbalancers",
				ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
			},
		},
	}

	// Unless static nodes are being deleted in which case a cleanup
	// needs to be executed that removes claudie installed utilities.
	if opts.IsStatic {
		ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, &spec.StageAnsibler_SubPass{
			Kind: spec.StageAnsibler_REMOVE_CLAUDIE_UTILITIES,
			Description: &spec.StageDescription{
				About: "Removing claudie install utilities",
				// Failing to remove the utilities is not considered as a fatal error.
				ErrorLevel: spec.ErrorLevel_ERROR_WARN,
			},
		})
	}

	// Unless the proxy is in use, in which case the task needs to also
	// update proxy environment variables after deletion, in which case
	// the task will also bundle the update of the VPN as there is a call
	// to be made to the Ansibler stage.
	if opts.UseProxy {
		ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, []*spec.StageAnsibler_SubPass{
			{
				Kind: spec.StageAnsibler_INSTALL_VPN,
				Description: &spec.StageDescription{
					About:      "Refreshing VPN on nodes after deletion",
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
		}...)
	}

	if len(ans.Ansibler.SubPasses) > 0 {
		pipeline = append(pipeline, &spec.Stage{StageKind: &ans})
	}

	// Only send the data through the terraformer stage if deletion is
	// for dynamic nodes.
	if !opts.IsStatic {
		pipeline = append(pipeline, &spec.Stage{
			StageKind: &spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Reconciling infrastructure for kubernetes cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      "Destroying VMs for deleted nodes",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		})
	}

	for np, nodes := range diff.PartiallyDeleted {
		// If the ApiServer is on the kubernetes cluster on deletion of the
		// control plane nodes the Kubeadm config map needs to be updated which
		// is used during the join operation of new nodes.
		if opts.HasApiServer && nodepools.FindByName(np, current.K8S.ClusterInfo.NodePools).IsControl {
			kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, []*spec.StageKuber_SubPass{
				{
					Kind: spec.StageKuber_PATCH_KUBEADM,
					Description: &spec.StageDescription{
						About:      "Updating Kubeadm certSANs",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
				},
				{
					Kind: spec.StageKuber_CILIUM_RESTART,
					Description: &spec.StageDescription{
						About: "Rollout restart of cilium pods",
						// Rollout restart failing is not a fatal error.
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
				},
			}...)
		}

		// If changes to the nodepool affects any loadbalancer
		// also schedule a reconciliation of loadbalancers, as
		// the envoy targets needs to be regenerated.
		for _, lb := range current.LoadBalancers.Clusters {
			for _, r := range lb.Roles {
				for _, tg := range r.TargetPools {
					// Need to match against only the nodepool name without the hash.
					if n, _ := nodepools.MatchNameAndHashWithTemplate(tg, np); n != "" {
						ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, &spec.StageAnsibler_SubPass{
							Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
							Description: &spec.StageDescription{
								About:      "Reconciling Envoy settings after changes to nodepool",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						})
					}
				}
			}
		}

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
						Delta: &spec.Update_KDeleteNodes{
							KDeleteNodes: &spec.Update_KuberDeleteK8SNodes{
								WithNodePool: false,
								Nodepool:     np,
								Nodes:        nodes,
								Unreachable:  opts.Unreachable,
							},
						},
					},
				},
			},
			Description: fmt.Sprintf("Deleting %v nodes from nodepool %s", len(nodes), np),
			Pipeline:    pipeline,
		}
	}

	for np, nodes := range diff.Deleted {
		// If the ApiServer is on the kubernetes cluster on deletion of the
		// control plane nodes the Kubeadm config map needs to be updated which
		// is used during the join operation of new nodes.
		if opts.HasApiServer && nodepools.FindByName(np, current.K8S.ClusterInfo.NodePools).IsControl {
			kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, []*spec.StageKuber_SubPass{
				{
					Kind: spec.StageKuber_PATCH_KUBEADM,
					Description: &spec.StageDescription{
						About:      "Updating Kubeadm certSANs",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
				},
				{
					Kind: spec.StageKuber_CILIUM_RESTART,
					Description: &spec.StageDescription{
						About: "Rollout restart of cilium pods",
						// Rollout restart failing is not a fatal error.
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
				},
			}...)
		}

		// If the deletion of the last autoscaled nodepools is to be scheduled. Also remove
		// the CA requirement for the cluster.
		if a := nodepools.Autoscaled(current.K8S.ClusterInfo.NodePools); len(a) == 1 {
			if a[0].Name == np {
				kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, &spec.StageKuber_SubPass{
					Kind: spec.StageKuber_DISABLE_LONGHORN_CA,
					Description: &spec.StageDescription{
						About:      "Disabling Longhorn cluster autoscaler setting",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
				})
			}
		}

		// On deletion of the whole nodepool, reconcile the storage classes for longhorn.
		kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, &spec.StageKuber_SubPass{
			Kind: spec.StageKuber_RECONCILE_LONGHORN_STORAGE_CLASSES,
			Description: &spec.StageDescription{
				About:      "Reconciling claudie longhorn storage classes after nodepool deletion",
				ErrorLevel: spec.ErrorLevel_ERROR_WARN,
			},
		})

		// If changes to the nodepool affects any loadbalancer
		// also schedule a reconciliation of loadbalancers, as
		// the envoy targets needs to be regenerated.
		for _, lb := range current.LoadBalancers.Clusters {
			for _, r := range lb.Roles {
				for _, tg := range r.TargetPools {
					// Need to match against only the nodepool name without the hash.
					if n, _ := nodepools.MatchNameAndHashWithTemplate(tg, np); n != "" {
						ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, &spec.StageAnsibler_SubPass{
							Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
							Description: &spec.StageDescription{
								About:      "Reconciling Envoy settings after changes to nodepool",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						})
					}
				}
			}
		}

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
						Delta: &spec.Update_KDeleteNodes{
							KDeleteNodes: &spec.Update_KuberDeleteK8SNodes{
								WithNodePool: true,
								Nodepool:     np,
								Nodes:        nodes,
								Unreachable:  opts.Unreachable,
							},
						},
					},
				},
			},
			Description: fmt.Sprintf("Deleting nodepool %s", np),
			Pipeline:    pipeline,
		}
	}

	return nil
}
