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
	tracker Tracker,
) {
	action, ok := tracker.Task.Do.(*spec.Task_Delete)
	if !ok {
		logger.
			Warn().
			Msgf("received task with action %T while wanting to destroy kubernetes cluster, assuming the task was misscheduled, ignoring", tracker.Task.Do)
		return
	}

	toDelete := action.Delete

	logger.Info().Msgf("Destroying kubernetes cluster")

	var loadbalancerApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpoint(toDelete.LoadBalancers); ep != nil {
		loadbalancerApiEndpoint = ep.Dns.Endpoint
	}

	k := kube_eleven.KubeEleven{
		K8sCluster:           toDelete.K8S,
		LoadBalancerEndpoint: loadbalancerApiEndpoint,
		SpawnProcessLimit:    processLimit,
	}

	if err := k.DestroyCluster(); err != nil {
		logger.Error().Msgf("Error while destroying cluster: %s", err)
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msgf("Kubernetes cluster was successfully destroyed")

	// No changes to LoadBalancers, update only kuberentes cluster.
	toDelete.K8S.Kubeconfig = ""
	update := tracker.Result.Update()
	update.Kubernetes(toDelete.K8S)
	update.Commit()
}
