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

const removeProxy = "../../ansible-playbooks/proxy/remove-proxy-envs.yml"

// ClearProxyEnvs updates the environment of the nodes to clear the proxy environment
// variables. These changes are not fully committed to yet, until the [CommitProxyEnvs]
// function is called.
func ClearProxyEnvs(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.
		Info().
		Msg("Clearing Proxy Envs on cluster nodes")

	update, ok := tracker.Task.Do.(*spec.TaskV2_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to clear proxy envs, assuming task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	state := update.Update.State
	proxy := utils.HttpProxyUrlAndNoProxyList(state.K8S, state.LoadBalancers)
	if err := clearProxyEnvsOnNodes(state.K8S, proxy, processLimit); err != nil {
		logger.Err(err).Msg("Failed to clear proxy envs")
		tracker.Diagnostics.Push(err)
		return
	}

	log.
		Info().
		Msgf("Successfully cleared proxy envs for nodes in cluster")
}

func clearProxyEnvsOnNodes(cluster *spec.K8SclusterV2, proxy utils.Proxy, processLimit *semaphore.Weighted) error {
	clusterID := cluster.ClusterInfo.Id()

	// This is the directory where files (Ansible inventory files, SSH keys etc.) will be generated.
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

	if err := nodepools.DynamicGenerateKeys(nodepools.Dynamic(cluster.ClusterInfo.NodePools), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := nodepools.StaticGenerateKeys(nodepools.Static(cluster.ClusterInfo.NodePools), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	if err := utils.GenerateInventoryFile(templates.ProxyEnvsInventoryTemplate, clusterDirectory, utils.ProxyInventoryFileParameters{
		K8sNodepools: utils.NodePools{
			Dynamic: nodepools.Dynamic(cluster.ClusterInfo.NodePools),
			Static:  nodepools.Static(cluster.ClusterInfo.NodePools),
		},
		ClusterID:    clusterID,
		NoProxyList:  proxy.NoProxyList,
		HttpProxyUrl: proxy.HttpProxyUrl,
	}); err != nil {
		return fmt.Errorf("failed to generate inventory file for updating proxy envs in /etc/environment using playbook in %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
		Playbook:          removeProxy,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Update proxy envs in /etc/environment - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible to update proxy envs /etc/environment in %s : %w", clusterDirectory, err)
	}

	return nil
}
