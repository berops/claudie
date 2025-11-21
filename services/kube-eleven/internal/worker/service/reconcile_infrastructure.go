package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/proto/pb/spec"
	kube_eleven "github.com/berops/claudie/services/kube-eleven/internal/worker/service/internal/kube-eleven"
	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"
)

// Renconciles the infrastructure to form a kubernetes cluster.
func Reconcile(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	task *spec.TaskV2,
	tracker Tracker,
) {
	var k8s *spec.K8SclusterV2
	var lbs []*spec.LBclusterV2

	switch task := task.Do.(type) {
	case *spec.TaskV2_Create:
		k8s = task.Create.K8S
		lbs = task.Create.LoadBalancers
	case *spec.TaskV2_Update:
		k8s = task.Update.State.K8S
		lbs = task.Update.State.LoadBalancers
	default:
		logger.
			Warn().
			Msgf("received task with action %T while wanting to reconcile kubernetes cluster, assuming the task was misscheduled, ignoring", task)
		tracker.Result.KeepAsIs()
		return
	}

	logger.Info().Msgf("Reconciling kubernetes cluster")

	var loadbalancerApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpointV2(lbs); ep != nil {
		loadbalancerApiEndpoint = ep.Dns.Endpoint
	}

	k := kube_eleven.KubeEleven{
		K8sCluster:           k8s,
		LoadBalancerEndpoint: loadbalancerApiEndpoint,
		SpawnProcessLimit:    processLimit,
	}

	if err := k.BuildCluster(); err != nil {
		logger.Err(err).Msg("Failed to reconcile cluster")
		tracker.Diagnostics.Push(err.Error())
		tracker.Result.KeepAsIs()
		return
	}

	logger.Info().Msg("Successfully reconciled kubernetes cluster")
	tracker.
		Result.
		ToUpdate().
		TakeKubernetesCluster(k8s).
		TakeLoadBalancers(lbs...).
		Replace()
}
