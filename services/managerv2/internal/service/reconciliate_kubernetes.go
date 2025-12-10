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

// TODO: finish.

// KubernetesModifications returns kubernetes cluster changes that can be done/executed before
// handling any deletion changes of clusters either, kubernetes of loadbalancers. Assumes that
// both the current and desired [spec.Clusters] were not modified since the [HealthCheckStatus]
// and [KubernetesDiffResult] was computed, and that all of the Cached Indices within the
// [KubernetesDiffResult] are not invalidated. This function does not modify the input in any
// ay and also the returned [spec.TaskEvent] does not hold or share any memory to related to the input.
func KubernetesModifications(
	hc *HealthCheckStatus,
	diff *KubernetesDiffResult,
	current *spec.ClustersV2,
	desired *spec.ClustersV2,
) *spec.TaskEventV2 {
	if diff.ApiEndpoint.Current != "" && diff.ApiEndpoint.Desired != "" {
		// make sure the desired node is already in the current state.
		transfer := diff.ApiEndpoint.Current != diff.ApiEndpoint.Desired
		transfer = transfer && nodepools.ContainsNode(current.K8S.ClusterInfo.NodePools, diff.ApiEndpoint.Desired)
		if transfer {
			return ScheduleTransferApiEndpoint(current, diff.ApiEndpoint.DesiredNodePool, diff.ApiEndpoint.Desired)
		}
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
func KubernetesDeletions(
	hc *HealthCheckStatus,
	diff *KubernetesDiffResult,
	current *spec.ClustersV2,
	desired *spec.ClustersV2,
) *spec.TaskEventV2 {
	return nil
}

// KubernetesLowPriority handles the very last low priority tasks that should be worked on,
// after all the other changes are done. Assumes that both the current and desired [spec.Clusters]
// were not modified since the [HealthCheckStatus] and [KubernetesDiffResult] was computed,
// and that all of the Cached Indices within the [KubernetesDiffResult] are not invalidated.
// This function does not modify the input in any way and also the returned [spec.TaskEvent]
// does not hold or share any memory to related to the input.
func KubernetesLowPriority(
	diff *KubernetesDiffResult,
	current *spec.ClustersV2,
	desired *spec.ClustersV2,
) *spec.TaskEventV2 {
	if diff.KubernetesVersion {
		return ScheduleUpgradeKubernetesVersion(current, desired)
	}

	switch diff.Proxy.Change {
	case ProxyOff:
		return ScheduleProxyOff(current, desired)
	case ProxyOn:
		return ScheduleProxyOn(current, desired)
	}

	labels := len(diff.LabelsTaintsAnnotations.Added.LabelKeys) > 0
	labels = labels || len(diff.LabelsTaintsAnnotations.Deleted.LabelKeys) > 0

	annotations := len(diff.LabelsTaintsAnnotations.Added.AnnotationsKeys) > 0
	annotations = annotations || len(diff.LabelsTaintsAnnotations.Deleted.AnnotationsKeys) > 0

	taints := len(diff.LabelsTaintsAnnotations.Added.TaintKeys) > 0
	taints = taints || len(diff.LabelsTaintsAnnotations.Deleted.TaintKeys) > 0

	if labels || annotations || taints {
		return SchedulePatchNodes(current, diff.LabelsTaintsAnnotations)
	}

	return nil
}

// Schedules a task that will update the kubernetes version to the new desired version of the cluster.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleUpgradeKubernetesVersion(current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	// TODO: this cannot be rollbacked once the rollback mechanism is implemented.
	// same way deletion shouldn't be rolled back on error. Same way The Change
	// of the Api Endpoint shouldn't be rolled back, those should be tried
	// infinitely or just ignore errors as some point ?

	inFlight := proto.Clone(current).(*spec.ClustersV2)
	toReplace := desired.K8S.Kubernetes

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
					Delta: &spec.UpdateV2_UpgradeVersion_{
						UpgradeVersion: &spec.UpdateV2_UpgradeVersion{
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
func ScheduleProxyOff(current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	toReplace := proto.Clone(desired.K8S.InstallationProxy).(*spec.InstallationProxyV2)
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
					Delta: &spec.UpdateV2_AnsReplaceProxy{
						AnsReplaceProxy: &spec.UpdateV2_AnsiblerReplaceProxySettings{
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
									About:      "Commiting proxy environment variables",
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
func ScheduleProxyOn(current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	toReplace := proto.Clone(desired.K8S.InstallationProxy).(*spec.InstallationProxyV2)
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
					Delta: &spec.UpdateV2_AnsReplaceProxy{
						AnsReplaceProxy: &spec.UpdateV2_AnsiblerReplaceProxySettings{
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
									About:      "Commiting proxy environment variables",
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
func SchedulePatchNodes(current *spec.ClustersV2, diff LabelsTaintsAnnotationsDiffResult) *spec.TaskEventV2 {
	var (
		inFlight = proto.Clone(current).(*spec.ClustersV2)
		toPatch  = spec.UpdateV2_KuberPatchNodes{}
	)

	for np, keys := range diff.Added.LabelKeys {
		toPatch.Add.Labels[np] = &spec.UpdateV2_KuberPatchNodes_ListOfLabelKeys{
			Labels: keys,
		}
	}

	for np, keys := range diff.Added.AnnotationsKeys {
		toPatch.Add.Annotations[np] = &spec.UpdateV2_KuberPatchNodes_ListOfAnnotationKeys{
			Annotations: keys,
		}
	}

	for np, taints := range diff.Added.TaintKeys {
		toPatch.Add.Taints[np] = &spec.UpdateV2_KuberPatchNodes_ListOfTaintKeys{
			Taints: taints,
		}
	}

	for np, keys := range diff.Deleted.LabelKeys {
		toPatch.Remove.Labels[np] = &spec.UpdateV2_KuberPatchNodes_ListOfLabelKeys{
			Labels: keys,
		}
	}

	for np, keys := range diff.Deleted.AnnotationsKeys {
		toPatch.Remove.Annotations[np] = &spec.UpdateV2_KuberPatchNodes_ListOfAnnotationKeys{
			Annotations: keys,
		}
	}

	for np, taints := range diff.Deleted.TaintKeys {
		toPatch.Remove.Taints[np] = &spec.UpdateV2_KuberPatchNodes_ListOfTaintKeys{
			Taints: taints,
		}
	}

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
					Delta: &spec.UpdateV2_KpatchNodes{
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

// Schedules a task that transfer the Api endpoint from the current node of the kubernetes
// cluster to the new desired node within the cluster
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleTransferApiEndpoint(current *spec.ClustersV2, nodepool, node string) *spec.TaskEventV2 {
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
					Delta: &spec.UpdateV2_K8SApiEndpoint{
						K8SApiEndpoint: &spec.UpdateV2_K8SOnlyApiEndpoint{
							Nodepool: nodepool,
							Node:     node,
						},
					},
				},
			},
		},
		Description: fmt.Sprintf("Transfering Api endpoint to %s from nodepool %s", node, nodepool),
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
									About:      "Transfering api endpoint",
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
