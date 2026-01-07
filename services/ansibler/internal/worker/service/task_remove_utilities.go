package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	utils "github.com/berops/claudie/services/ansibler/internal/worker/service/internal"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

const wireguardUninstall = "../../ansible-playbooks/wireguard-uninstall.yml"

// RemoveUtilities removes claudie installed utilities.
//
// The cleaning will only be performed on static nodepools
// as dynamic nodepools are transient and managed by claudie
// and can be replaced any time by claudie itself.
func RemoveUtilities(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Removing Claudie utilities")

	var (
		npi          []*NodepoolsInfo
		k8sClusterId string
	)

	switch do := tracker.Task.Do.(type) {
	case *spec.Task_Update:
		n, ok := nodepoolsInfoUpdate(logger, do)
		if !ok {
			return
		}
		npi = append(npi, n...)
		k8sClusterId = do.Update.State.K8S.ClusterInfo.Id()
	case *spec.Task_Delete:
		npi = append(npi, nodepoolsInfoDelete(do)...)
		k8sClusterId = do.Delete.K8S.ClusterInfo.Id()
	default:
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to remove claudie utilities, assuming task was misscheduled, ignoring", do)
		return
	}

	if len(npi) == 0 {
		// No static nodes to cleanup.
		return
	}

	logger.Info().Msgf("Starting cleanup of utilities installed by Claudie, may take a while")

	if err := removeUtilities(k8sClusterId, npi, processLimit); err != nil {
		logger.Err(err).Msg("Failed to remove claudie utilities from nodes of the cluster")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Successfully removed claudie utilities")
}

func removeUtilities(clusterID string, npi []*NodepoolsInfo, processLimit *semaphore.Weighted) error {
	clusterDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)),
	)

	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", clusterDirectory, err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDirectory); err != nil {
			log.Err(err).Msgf("error while deleting files in %s", clusterDirectory)
		}
	}()

	err := utils.GenerateInventoryFile(
		templates.AllNodesInventoryTemplate,
		clusterDirectory,
		AllNodesInventoryData{
			NodepoolsInfo: npi,
		},
	)
	if err != nil {
		return fmt.Errorf("error while creating inventory file for %s: %w", clusterDirectory, err)
	}

	for _, npi := range npi {
		if err := nodepools.DynamicGenerateKeys(npi.Nodepools.Dynamic, clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
		}

		if err := nodepools.StaticGenerateKeys(npi.Nodepools.Static, clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
		}
	}

	ansible := utils.Ansible{
		// since removing claudie utilities should only be used on nodes that are either deleted, or the cluster is being
		// deleted, have a retry count of just 1 times of connecting to each node. It can be the case that the user manually
		// deleted the infrastructure and thus some of the nodes may not even be reachable. The other case, in which some of the
		// nodes are not reachable it would also make no sense in having a higher retry count.
		// However if there are connectivity issuess, some of the nodes may not be properly clean-up, but if the same nodes
		// would then re-join a new claudie made cluster it would complain (mostly about the kubernetes binaries), and the user
		// would have to clean it up manually.
		RetryCount:        1,
		Playbook:          wireguardUninstall,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	// Subsequent calling may fail, thus simply log the error.
	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Remove Utils - %s", clusterID)); err != nil {
		log.Warn().Msgf("error while uninstalling wireguard ansible for %s : %s", clusterID, err)
	}

	return nil
}

func nodepoolsInfoDelete(task *spec.Task_Delete) []*NodepoolsInfo {
	var npi []*NodepoolsInfo

	k8s := &NodepoolsInfo{
		Nodepools: NodePools{
			// ignore dynamic nodepools, as they're managed by claudie and not externally.
			Dynamic: []*spec.NodePool{},
			Static:  nodepools.Static(task.Delete.K8S.ClusterInfo.NodePools),
		},
		ClusterID:      task.Delete.K8S.ClusterInfo.Id(),
		ClusterNetwork: task.Delete.K8S.Network,
	}

	if len(k8s.Nodepools.Static) > 0 {
		npi = append(npi, k8s)
	}

	for _, lb := range task.Delete.LoadBalancers {
		lb := &NodepoolsInfo{
			Nodepools: NodePools{
				// ignore dynamic nodepools, as they're managed by claudie and not externally.
				Dynamic: []*spec.NodePool{},
				Static:  nodepools.Static(lb.ClusterInfo.NodePools),
			},
			ClusterID:      lb.ClusterInfo.Id(),
			ClusterNetwork: task.Delete.K8S.Network,
		}

		if len(lb.Nodepools.Static) > 0 {
			npi = append(npi, lb)
		}
	}

	return npi
}

func nodepoolsInfoUpdate(logger zerolog.Logger, task *spec.Task_Update) ([]*NodepoolsInfo, bool) {
	var npi []*NodepoolsInfo

	switch delta := task.Update.Delta.(type) {
	case *spec.Update_DeletedK8SNodes_:
		np := DefaultKubernetesToDeletedNodesOnly(task.Update.State.K8S, delta.DeletedK8SNodes)

		// On valid scheduled messages, the nodepool from which
		// nodes are to be deleted, should always be present in the
		// provided state.
		if np == nil {
			log.
				Warn().
				Msgf("Received update task for removal of claudie installed utilities on deleted nodes of the cluster %q, but the nodepool from which nodes were deleted is not in the provided state, ignoring",
					task.Update.State.K8S.ClusterInfo.Id(),
				)
			return nil, false
		}

		static := nodepools.Static([]*spec.NodePool{np})

		// This task may be called during the deletion of unreachable nodes
		// thus filter them out when processing.
		if delta.DeletedK8SNodes.Unreachable != nil {
			static = DefaultNodePoolsToReachableInfrastructureOnly(
				static,
				delta.DeletedK8SNodes.Unreachable.Kubernetes,
			)
		}

		k8s := &NodepoolsInfo{
			Nodepools: NodePools{
				// ignore dynamic nodepools, as they're managed by claudie and not externally.
				Dynamic: []*spec.NodePool{},
				Static:  static,
			},
			ClusterID:      task.Update.State.K8S.ClusterInfo.Id(),
			ClusterNetwork: task.Update.State.K8S.Network,
		}

		if len(k8s.Nodepools.Static) > 0 {
			npi = append(npi, k8s)
		}
	case *spec.Update_DeletedLoadBalancerNodes_:
		handle := delta.DeletedLoadBalancerNodes.Handle
		idx := clusters.IndexLoadbalancerById(handle, task.Update.State.LoadBalancers)
		if idx < 0 {
			log.
				Warn().
				Msgf("Received update task for removal of claudie installed utilities on deleted loadbalancer nodes but the loadbalancer %q is not in the provided state, ignoring", handle)
			return nil, false
		}

		lb := task.Update.State.LoadBalancers[idx]
		np := DefaultLoadBalancerToDeletedNodesOnly(lb, delta.DeletedLoadBalancerNodes)

		// On valid scheduled messages, the nodepool from which
		// nodes are to be deleted, should always be present in the
		// provided state.
		if np == nil {
			log.
				Warn().
				Msgf("Received update task for removal of claudie installed utilities on deleted nodes of the cluster %q, but the nodepool from which nodes were deleted is not in the provided state, ignoring",
					lb.ClusterInfo.Id(),
				)
			return nil, false
		}

		static := nodepools.Static([]*spec.NodePool{np})

		// This task may be called during the deletion of unreachable nodes
		// thus filter them out when processing.
		if delta.DeletedLoadBalancerNodes.Unreachable != nil {
			static = DefaultNodePoolsToReachableInfrastructureOnly(
				static,
				delta.DeletedLoadBalancerNodes.Unreachable.Loadbalancers[handle],
			)
		}

		lbi := &NodepoolsInfo{
			Nodepools: NodePools{
				// ignore dynamic nodepools, as they're managed by claudie and not externally.
				Dynamic: []*spec.NodePool{},
				Static:  static,
			},
			ClusterID:      lb.ClusterInfo.Id(),
			ClusterNetwork: task.Update.State.K8S.Network,
		}

		if len(lbi.Nodepools.Static) > 0 {
			npi = append(npi, lbi)
		}
	case *spec.Update_DeleteLoadBalancer_:
		handle := delta.DeleteLoadBalancer.Handle
		idx := clusters.IndexLoadbalancerById(handle, task.Update.State.LoadBalancers)
		if idx < 0 {
			log.
				Warn().
				Msgf("Received update task for removal of claudie installed utilities on deleted loadbalancer but the loadbalancer %q is not in the provided state, ignoring", handle)
			return nil, false
		}

		lb := task.Update.State.LoadBalancers[idx]
		static := nodepools.Static(lb.ClusterInfo.NodePools)

		// This task may be called during the deletion of unreachable nodes
		// thus filter them out when processing.
		if delta.DeleteLoadBalancer.Unreachable != nil {
			static = DefaultNodePoolsToReachableInfrastructureOnly(
				static,
				delta.DeleteLoadBalancer.Unreachable.Loadbalancers[handle],
			)
		}

		lbi := &NodepoolsInfo{
			Nodepools: NodePools{
				// ignore dynamic nodepools, as they're managed by claudie and not externally.
				Dynamic: []*spec.NodePool{},
				Static:  static,
			},
			ClusterID:      lb.ClusterInfo.Id(),
			ClusterNetwork: task.Update.State.K8S.Network,
		}

		if len(lbi.Nodepools.Static) > 0 {
			npi = append(npi, lbi)
		}
	default:
		logger.
			Warn().
			Msgf("Received update task with delta %T while wanting to remove claudie utilities, assuming task was misscheduled, ignoring", delta)
		return nil, false
	}

	return npi, true
}
