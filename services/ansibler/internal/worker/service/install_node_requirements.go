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
	"golang.org/x/sync/semaphore"
)

const nodeRequirementsPlayBook = "../../ansible-playbooks/longhorn-req.yml"

// InstallNodeRequirements install pre-requisite tools on all nodes, of the kubernetes cluster,
// that are required by Claudie to operate.
func InstallNodeRequirements(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	task *spec.TaskV2,
	tracker Tracker,
) {
	logger.Info().Msg("Installing node requirements")
	k8s, _, ok := utils.StateFromTask(task)
	if !ok {
		logger.
			Warn().
			Msgf("received task with action %T while wanting to install node requirements, assuming the task was misscheduled, ignoring", task.GetDo())
		tracker.Result.KeepAsIs()
		return
	}

	ni := NodepoolsInfo{
		Nodepools: utils.NodePools{
			Dynamic: nodepools.Dynamic(k8s.ClusterInfo.NodePools),
			Static:  nodepools.Static(k8s.ClusterInfo.NodePools),
		},
		ClusterID:      k8s.ClusterInfo.Id(),
		ClusterNetwork: k8s.Network,
	}

	if err := installLonghornRequirements(&ni, processLimit); err != nil {
		tracker.Diagnostics.Push(err.Error())
		tracker.Result.KeepAsIs()
		return
	}

	// Installing node requirements does not change the cluster state in any way.
	tracker.Result.KeepAsIs()
}

// installLonghornRequirements installs pre-requisite tools for LongHorn in all the nodes
func installLonghornRequirements(nodepoolsInfo *NodepoolsInfo, processLimit *semaphore.Weighted) error {
	clusterDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s", nodepoolsInfo.ClusterID, hash.Create(hash.Length)),
	)

	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	if err := nodepools.DynamicGenerateKeys(nodepoolsInfo.Nodepools.Dynamic, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools: %w", err)
	}

	if err := nodepools.StaticGenerateKeys(nodepoolsInfo.Nodepools.Static, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	data := AllNodesInventoryData{
		NodepoolsInfo: []*NodepoolsInfo{nodepoolsInfo},
	}

	if err := utils.GenerateInventoryFile(templates.AllNodesInventoryTemplate, clusterDirectory, data); err != nil {
		return fmt.Errorf("failed to generate inventory file for all nodes in %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:          nodeRequirementsPlayBook,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Node requirements - %s", nodepoolsInfo.ClusterID)); err != nil {
		return fmt.Errorf("error while running ansible playbook at %s to install Longhorn requirements : %w", clusterDirectory, err)
	}

	return os.RemoveAll(clusterDirectory)
}
