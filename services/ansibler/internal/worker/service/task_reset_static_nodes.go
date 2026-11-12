package service

import (
	"fmt"
	"os"
	"path/filepath"

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

// ResetStaticNodes resets the static nodes of the Kubernetes cluster from the delete task
// and removes the Kubernetes packages from them.
//
// The dynamic nodes are skipped, they are destroyed together with the infrastructure
// of the cluster right after.
func ResetStaticNodes(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Resetting static nodes of the Kubernetes cluster")

	del, ok := tracker.Task.Do.(*spec.Task_Delete)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to reset static nodes of the kubernetes cluster, assuming the task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	k8s := del.Delete.K8S

	static := nodepools.Static(k8s.ClusterInfo.NodePools)
	if len(static) == 0 {
		logger.Info().Msg("No static nodes in the cluster, nothing to reset")
		return
	}

	if err := resetStaticNodes(k8s.ClusterInfo.Id(), static, processLimit); err != nil {
		logger.Err(err).Msg("Failed to reset the static nodes of the Kubernetes cluster")
		tracker.Diagnostics.Push(err)
		return
	}

	// No changes to the loadbalancers, the cluster has no kubeconfig anymore.
	k8s.Kubeconfig = ""
	update := tracker.Result.Update()
	update.Kubernetes(k8s)
	update.Commit()

	logger.Info().Msg("Finished resetting static nodes of the Kubernetes cluster")
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
