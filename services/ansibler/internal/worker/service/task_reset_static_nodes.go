package service

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

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

const resetStaticNodesPlaybook = "../../ansible-playbooks/reset-static-nodes.yml"

// ResetStaticNodes resets the static nodes that leave the Kubernetes cluster and removes the
// Kubernetes packages from them: all static nodes of the cluster on a delete task, the deleted
// static nodes on an update task.
//
// The dynamic nodes are skipped, they are destroyed together with their infrastructure.
func ResetStaticNodes(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Resetting static nodes of the Kubernetes cluster")

	var (
		k8s    *spec.K8Scluster
		static []*spec.NodePool
	)

	switch do := tracker.Task.Do.(type) {
	case *spec.Task_Delete:
		k8s = do.Delete.K8S
		static = nodepools.Static(k8s.ClusterInfo.NodePools)
	case *spec.Task_Update:
		deleted := do.Update.GetDeletedK8SNodes()
		if deleted == nil {
			logger.
				Warn().
				Msgf("Received update task with delta %T while wanting to reset static nodes of the kubernetes cluster, assuming the task was misscheduled, ignoring", do.Update.GetDelta())
			return
		}
		k8s = do.Update.State.K8S
		static = deletedStaticNodes(k8s, deleted)
	default:
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to reset static nodes of the kubernetes cluster, assuming the task was misscheduled, ignoring", do)
		return
	}

	if !slices.ContainsFunc(static, func(np *spec.NodePool) bool { return len(np.Nodes) > 0 }) {
		logger.Info().Msg("No reachable static nodes to reset")
		return
	}

	if err := resetStaticNodes(k8s.ClusterInfo.Id(), static, processLimit); err != nil {
		logger.Err(err).Msg("Failed to reset the static nodes of the Kubernetes cluster")
		tracker.Diagnostics.Push(err)
		return
	}

	// The cluster of an update task keeps running, with the same kubeconfig.
	if _, ok := tracker.Task.Do.(*spec.Task_Delete); ok {
		// No changes to the loadbalancers, the cluster has no kubeconfig anymore.
		k8s.Kubeconfig = ""
		update := tracker.Result.Update()
		update.Kubernetes(k8s)
		update.Commit()
	}

	logger.Info().Msg("Finished resetting static nodes of the Kubernetes cluster")
}

// deletedStaticNodes returns the reachable static nodes deleted from the cluster, in a copy
// of their nodepool. The nodes are no longer part of the nodepools of the cluster.
func deletedStaticNodes(k8s *spec.K8Scluster, deleted *spec.Update_DeletedK8SNodes) []*spec.NodePool {
	np := DefaultKubernetesToDeletedNodesOnly(k8s, deleted)
	if np == nil {
		return nil
	}

	static := nodepools.Static([]*spec.NodePool{np})
	if deleted.Unreachable != nil {
		static = DefaultNodePoolsToReachableInfrastructureOnly(static, deleted.Unreachable.Kubernetes)
	}

	return static
}

// resetStaticNodes runs the reset-static-nodes playbook against the passed in static nodepools. The
// playbook does not touch the etcd membership, so the nodes must be either all nodes of a destroyed
// cluster or nodes that kuber already deleted from the cluster.
func resetStaticNodes(clusterID string, static []*spec.NodePool, processLimit *semaphore.Weighted) error {
	clusterDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)),
	)

	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDirectory); err != nil {
			log.Err(err).Msgf("error while deleting files in %s", clusterDirectory)
		}
	}()

	// The inventory references the per-node .pem files, they need to exist in the cluster directory.
	if err := nodepools.StaticGenerateKeys(static, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	idata := KubernetesInventoryParameters{
		K8sNodepools: NodePools{
			Static: static,
		},
		ClusterID: clusterID,
	}

	if err := utils.GenerateInventoryFile(templates.KubernetesInventoryTemplate, clusterDirectory, idata); err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:          resetStaticNodesPlaybook,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Reset static nodes - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", clusterDirectory, err)
	}

	return nil
}
