package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
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
	tracker Tracker,
) {
	var k8s *spec.K8Scluster
	var lbs []*spec.LBcluster

	switch task := tracker.Task.Do.(type) {
	case *spec.Task_Create:
		k8s = task.Create.K8S
		lbs = task.Create.LoadBalancers
	case *spec.Task_Update:
		k8s = task.Update.State.K8S
		lbs = task.Update.State.LoadBalancers

		if upgrade := task.Update.GetUpgradeVersion(); upgrade != nil {
			logger.Info().Msg("Upgrading kubernetes version")
			k8s.Kubernetes = upgrade.Version
		}

		removeKubeProxy(logger, k8s.ClusterInfo.Id(), k8s.Kubeconfig)
	default:
		logger.
			Warn().
			Msgf("received task with action %T while wanting to reconcile kubernetes cluster, assuming the task was misscheduled, ignoring", task)
		return
	}

	logger.Info().Msgf("Reconciling kubernetes cluster")

	var loadbalancerApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpoint(lbs); ep != nil {
		loadbalancerApiEndpoint = ep.Dns.Endpoint
	}

	k := kube_eleven.KubeEleven{
		K8sCluster:           k8s,
		LoadBalancerEndpoint: loadbalancerApiEndpoint,
		SpawnProcessLimit:    processLimit,
	}

	if err := k.BuildCluster(); err != nil {
		logger.Err(err).Msg("Failed to reconcile cluster")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Successfully reconciled kubernetes cluster")

	// Mark all of the nodes with status Joined.
	for _, np := range k.K8sCluster.ClusterInfo.NodePools {
		for _, n := range np.Nodes {
			n.Status = spec.NodeStatus_Joined
		}
	}

	update := tracker.Result.Update()
	update.Kubernetes(k.K8sCluster)
	update.Commit()
}

// TODO: remove in future claudie versions.
// Only added in version v0.13.0 for backwards
// compatibility with versions v0.12.x with the
// move to ebpf cilium.
func removeKubeProxy(logger zerolog.Logger, clusterId, kubeconfig string) {
	k := kubectl.Kubectl{
		Kubeconfig:        kubeconfig,
		MaxKubectlRetries: 1,
	}

	k.Stdout = command.GetStdOut(clusterId)
	k.Stderr = command.GetStdErr(clusterId)

	var anyerror bool

	if err := k.KubectlDeleteResource("cm", "kube-proxy", "-n kube-system"); err != nil {
		anyerror = true
	}

	if err := k.KubectlDeleteResource("ds", "kube-proxy", "-n kube-system"); err != nil {
		anyerror = true
	}

	if anyerror {
		logger.
			Error().
			Msg("errors encountered while deleting kube-proxy, assuming kube-proxy is not deployed")
	}
}
