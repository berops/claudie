package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	kube_eleven "github.com/berops/claudie/services/kube-eleven/internal/worker/service/internal/kube-eleven"
	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"
)

// Destroys the kubernetes cluster on top of the provided
// infrastructure and uninstanll any related binaries.
func Destroy(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	task *spec.TaskV2,
	tracker Tracker,
) {
	action, ok := task.GetDo().(*spec.TaskV2_Delete)
	if !ok {
		logger.
			Warn().
			Msgf("received task with action %T while wanting to destroy kubernetes cluster, assuming the task was misscheduled, ignoring", task.GetDo())
		tracker.Result.KeepAsIs()
		return
	}

	delete, ok := action.Delete.GetOp().(*spec.DeleteV2_Clusters_)
	if !ok {
		logger.
			Warn().
			Msgf("received task with action %T while wanting to destroy kubernetes cluster, assuming the task was misscheduled, ignoring", action.Delete.GetOp())
		tracker.Result.KeepAsIs()
		return
	}

	logger.Info().Msgf("Destroying kubernetes cluster")

	var loadbalancerApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpointV2(delete.Clusters.LoadBalancers); ep != nil {
		loadbalancerApiEndpoint = ep.Dns.Endpoint
	}

	k := kube_eleven.KubeEleven{
		K8sCluster:           delete.Clusters.K8S,
		LoadBalancerEndpoint: loadbalancerApiEndpoint,
		SpawnProcessLimit:    processLimit,
	}

	if err := k.DestroyCluster(); err != nil {
		logger.Error().Msgf("Error while destroying cluster: %s", err)
		tracker.Diagnostics.Push(err.Error())
		tracker.Result.KeepAsIs()
		return
	}

	logger.Info().Msgf("Kubernetes cluster was successfully destroyed")
	delete.Clusters.K8S.Kubeconfig = ""
	tracker.
		Result.
		ToUpdate().
		TakeKubernetesCluster(delete.Clusters.K8S).
		TakeLoadBalancers(delete.Clusters.LoadBalancers...)
}
