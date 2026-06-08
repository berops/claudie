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

const ansibleTeeOverridePlaybookFilePath = "../../ansible-playbooks/tee-override.yml"

func InstallTeeOverride(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Installing tee binary override")

	var (
		nps                   []*spec.NodePool
		clusterId             string
		clusterPrivateNetwork string
	)

	switch do := tracker.Task.Do.(type) {
	case *spec.Task_Create:
		nps = do.Create.K8S.ClusterInfo.NodePools
		clusterId = do.Create.K8S.ClusterInfo.Id()
		clusterPrivateNetwork = do.Create.K8S.Network
	case *spec.Task_Update:
		clusterId = do.Update.State.K8S.ClusterInfo.Id()
		clusterPrivateNetwork = do.Update.State.K8S.Network

		// Try to only install the tee override on new nodes, if possible.
		// This is done to minimize the time for adding new nodes to an
		// existing cluster.
		if np := DefaultKubernetesToNewNodesIfPossible(do); np != nil {
			nps = []*spec.NodePool{np}
		} else {
			nps = do.Update.State.K8S.ClusterInfo.NodePools
		}
	default:
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to overrite tee binary, assuming task was misscheduled, ignoring", do)
		return
	}

	ni := NodepoolsInfo{
		Nodepools: NodePools{
			Dynamic: nodepools.Dynamic(nps),
			Static:  nodepools.Static(nps),
		},
		ClusterID:      clusterId,
		ClusterNetwork: clusterPrivateNetwork,
	}

	if err := installTeeOverride(&ni, processLimit); err != nil {
		logger.Err(err).Msg("Failed to deploy tee override")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Tee binary override finished")
}

// installTeeOverride injects the tee binary override on all the nodes
func installTeeOverride(nodepoolsInfo *NodepoolsInfo, processLimit *semaphore.Weighted) error {
	clusterDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s", nodepoolsInfo.ClusterID, hash.Create(hash.Length)),
	)

	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDirectory); err != nil {
			log.Err(err).Msgf("error while deleting files in %s", clusterDirectory)
		}
	}()

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
		Playbook:          ansibleTeeOverridePlaybookFilePath,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Install tee binary override - %s", nodepoolsInfo.ClusterID)); err != nil {
		return fmt.Errorf("error while running ansible playbook at %s to install tee binary override : %w", clusterDirectory, err)
	}

	return nil
}
