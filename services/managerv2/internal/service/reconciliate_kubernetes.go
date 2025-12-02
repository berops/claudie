package service

import "github.com/berops/claudie/proto/pb/spec"

// KubernetesModifications returns kubernetes cluster changes that can be done/executed before handling any deletion changes of clusters
// either, kubernetes of loadbalancers. Assumes that both the current and desired [spec.Clusters] were not modified since the [HealthCheckStatus]
// and [KubernetesDiffResult] was computed, and that all of the Cached Indices within the [KubernetesDiffResult] are not invalidated. This
// function does not modify the input in any way and also the returned [spec.TaskEvent] does not hold or shared any memory to related to the input.
func KubernetesModifications(hc *HealthCheckStatus, diff *KubernetesDiffResult, current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	return nil
}

// KubernetesDeletions returns kubernetes cluster changes can be done/executed after handling addition/modification changes to the kubernetes
// clusters, and deletions of loadbalancers. Assumes that both the current and desired [spec.Clusters] were not modified since the [HealthCheckStatus]
// and [KubernetesDiffResult] was computed, and that all of the Cached Indices within the [KubernetesDiffResult] are not invalidated. This function
// does not modify the input in any way and also the returned [spec.TaskEvent] does not hold or shared any memory to related to the input.
func KubernetesDeletions(hc *HealthCheckStatus, diff *KubernetesDiffResult, current, desired *spec.ClustersV2) *spec.TaskEventV2 {
	return nil
}
