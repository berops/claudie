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

const commitProxy = "../../ansible-playbooks/proxy/commit-proxy-envs-changes.yml"

// CommitProxyEnvs commits to using the proxy envs on related services by restarting them
// which results in refreshing their envs to consider the changed proxy settings.
func CommitProxyEnvs(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.
		Info().
		Msg("Updating kube-proxy DaemonSet and static pods with new Proxy envs")

	update, ok := tracker.Task.Do.(*spec.Task_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to commit proxy, assuming task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	state := update.Update.State
	if err := commitProxyEnvs(state.K8S, processLimit); err != nil {
		logger.Err(err).Msg("Failed to commit proxy envs")
		tracker.Diagnostics.Push(err)
		return
	}

	// Optionally, if  the task currently processed, needs to update the state,
	// update it with the passed in desired proxy setting.
	if updateProxy, ok := update.Update.Delta.(*spec.Update_AnsReplaceProxy); ok {
		state.K8S.InstallationProxy = updateProxy.AnsReplaceProxy.Proxy

		update := tracker.Result.Update()
		update.Kubernetes(state.K8S)
		update.Commit()
	}

	log.
		Info().
		Msg("Successfully updated proxy envs for kube-proxy DaemonSet and static pods ")
}

// commitProxyEnvs updates NO_PROXY and no_proxy envs across k8s services on nodes and restarts necessary
// services so that the changes will be propagated to them.
func commitProxyEnvs(cluster *spec.K8Scluster, processLimit *semaphore.Weighted) error {
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

	err := utils.GenerateInventoryFile(
		templates.ProxyEnvsInventoryTemplate,
		clusterDirectory,
		utils.ProxyInventoryFileParameters{
			K8sNodepools: utils.NodePools{
				Dynamic: nodepools.Dynamic(cluster.ClusterInfo.NodePools),
				Static:  nodepools.Static(cluster.ClusterInfo.NodePools),
			},
			ClusterID: clusterID,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to generate inventory file for updating proxy envs using playbook in %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:          commitProxy,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Update proxy envs - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible to update proxy envs in %s : %w", clusterDirectory, err)
	}

	return nil
}
