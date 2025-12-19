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

const populateProxy = "../../ansible-playbooks/proxy/populate-proxy-envs.yml"

// UpdateProxyEnvs updates the environment of the nodes. These changes are not
// ully committed to yet, until the [CommitProxyEnvs] function is called, unless
// this is called when creating the cluster in which case there is no need
// to call the [CommitProxyEnvs] func.
func UpdateProxyEnvs(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Updating Proxy Envs on cluster nodes")

	var k8s *spec.K8Scluster
	var lbs []*spec.LBcluster

	switch do := tracker.Task.Do.(type) {
	case *spec.Task_Create:
		k8s, lbs = do.Create.K8S, do.Create.LoadBalancers
	case *spec.Task_Update:
		k8s, lbs = do.Update.State.K8S, do.Update.State.LoadBalancers
	default:
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to update proxy envs, assuming task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	proxy := utils.HttpProxyUrlAndNoProxyList(k8s, lbs)
	if err := updateProxyEnvsOnNodes(k8s, proxy, processLimit); err != nil {
		logger.Err(err).Msg("Failed to update proxy envs")
		tracker.Diagnostics.Push(err)
		return
	}

	log.
		Info().
		Msgf("Successfully updated proxy envs for nodes in cluster")
}

func updateProxyEnvsOnNodes(cluster *spec.K8Scluster, proxy utils.Proxy, processLimit *semaphore.Weighted) error {
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
		Playbook:          populateProxy,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Update proxy envs in /etc/environment - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible to update proxy envs /etc/environment in %s : %w", clusterDirectory, err)
	}

	return nil
}
