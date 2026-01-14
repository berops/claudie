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
	clusterId := state.K8S.ClusterInfo.Id()

	// as the slice may be modified make a shallow copy of it,
	// while still allowing changes to be propagated back to the
	// state, if any.
	nps := slices.Clone(state.K8S.ClusterInfo.NodePools)

	if add := update.Update.GetAddedK8SNodes(); add != nil {
		// On addition, only commit on the existing, established nodes
		// as the new ones are not in the cluster yet.
		logger.Error().Msg("HERE, TODO: remove me after testing, PROXY ADD")

		if i := nodepools.IndexByName(add.Nodepool, nps); i >= 0 {
			np := DefaultNodePoolToExistingInfrastructureOnly(nps[i], add.Nodes)

			// The original nodepool won't be needed in this function, thus simply replace
			// it at the index of the cloned slice.
			nps[i] = np
		} else {
			logger.
				Warn().
				Msgf("Processing update for adding nodes to k8s cluster but the nodepool was not found, ignoring")
		}
	}

	// This task may be called during the deletion of unreachable nodes
	// thus filter them out when processing.
	if unreachable := UnreachableInfrastructure(update); unreachable != nil {
		logger.Error().Msg("HERE, TODO: remove me after testing")
		nps = DefaultNodePoolsToReachableInfrastructureOnly(
			nps,
			unreachable.Kubernetes,
		)
	}

	if err := commitProxyEnvs(clusterId, nps, processLimit); err != nil {
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
func commitProxyEnvs(clusterId string, nps []*spec.NodePool, processLimit *semaphore.Weighted) error {
	// This is the directory where files (Ansible inventory files, SSH keys etc.) will be generated.
	clusterDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s", clusterId, hash.Create(hash.Length)),
	)

	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	defer func() {
		if err := os.RemoveAll(clusterDirectory); err != nil {
			log.Err(err).Msgf("error while deleting files in %s", clusterDirectory)
		}
	}()

	if err := nodepools.DynamicGenerateKeys(nodepools.Dynamic(nps), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := nodepools.StaticGenerateKeys(nodepools.Static(nps), clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	err := utils.GenerateInventoryFile(
		templates.ProxyEnvsInventoryTemplate,
		clusterDirectory,
		ProxyInventoryFileParameters{
			K8sNodepools: NodePools{
				Dynamic: nodepools.Dynamic(nps),
				Static:  nodepools.Static(nps),
			},
			ClusterID: clusterId,
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

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Update proxy envs - %s", clusterId)); err != nil {
		return fmt.Errorf("error while running ansible to update proxy envs in %s : %w", clusterDirectory, err)
	}

	return nil
}
