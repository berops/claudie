package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

const containerdConfigPlaybook = "../../ansible-playbooks/containerd-config.yml"

// ReconcileContainerdConfig replaces the containerd drop-in on the nodes of the nodepool from the
// delta of the update task, of the kubernetes cluster or of a load balancer, one node at a time.
func ReconcileContainerdConfig(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Replacing containerd config")

	replace := tracker.Task.GetUpdate().GetReconcileContainerdConfig()
	if replace == nil {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to replace the containerd config, assuming the task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	var (
		state     = tracker.Task.GetUpdate().State
		clusterID = state.K8S.ClusterInfo.Id()
		nps       = state.K8S.ClusterInfo.NodePools
		lb        *spec.LBcluster
	)

	if replace.Handle != "" {
		idx := clusters.IndexLoadbalancerById(replace.Handle, state.LoadBalancers)
		if idx < 0 {
			err := fmt.Errorf("load balancer %q not found in cluster %s", replace.Handle, clusterID)
			logger.Err(err).Msg("Failed to replace the containerd config")
			tracker.Diagnostics.Push(err)
			return
		}
		lb = state.LoadBalancers[idx]
		clusterID = lb.ClusterInfo.Id()
		nps = lb.ClusterInfo.NodePools
	}

	np := nodepools.FindByName(replace.Nodepool, nps)
	if np == nil {
		err := fmt.Errorf("nodepool %q not found in cluster %s", replace.Nodepool, clusterID)
		logger.Err(err).Msg("Failed to replace the containerd config")
		tracker.Diagnostics.Push(err)
		return
	}
	np.ContainerdConfig = replace.Config

	if err := replaceContainerdConfig(clusterID, state.K8S.Network, np, processLimit); err != nil {
		logger.Err(err).Msg("Failed to replace the containerd config")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	if lb != nil {
		update.Loadbalancers(lb)
	} else {
		update.Kubernetes(state.K8S)
	}
	update.Commit()

	logger.Info().Msgf("Finished replacing the containerd config of nodepool %s", np.Name)
}

// replaceContainerdConfig runs the containerd-config playbook against the nodes of the nodepool.
func replaceContainerdConfig(clusterID, network string, np *spec.NodePool, processLimit *semaphore.Weighted) error {
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

	nps := []*spec.NodePool{np}
	ni := NodepoolsInfo{
		Nodepools: NodePools{
			Dynamic: nodepools.Dynamic(nps),
			Static:  nodepools.Static(nps),
		},
		ClusterID:      clusterID,
		ClusterNetwork: network,
	}

	if err := nodepools.DynamicGenerateKeys(ni.Nodepools.Dynamic, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := nodepools.StaticGenerateKeys(ni.Nodepools.Static, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	if err := writeContainerdConfigs(clusterDirectory, clusterID, nps); err != nil {
		return err
	}

	data := AllNodesInventoryData{NodepoolsInfo: []*NodepoolsInfo{&ni}}
	if err := utils.GenerateInventoryFile(templates.AllNodesInventoryTemplate, clusterDirectory, data); err != nil {
		return fmt.Errorf("failed to generate inventory file for all nodes in %s : %w", clusterDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:          containerdConfigPlaybook,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Containerd config - %s", clusterID)); err != nil {
		return fmt.Errorf("error while running ansible for %s : %w", clusterDirectory, err)
	}

	return nil
}

// writeContainerdConfigs writes the containerd drop-in of each node to containerd/<inventory host>.toml,
// read by the containerd_config role.
func writeContainerdConfigs(clusterDirectory, clusterID string, nps []*spec.NodePool) error {
	dir := filepath.Join(clusterDirectory, "containerd")
	if err := fileutils.CreateDirectory(dir); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", dir, err)
	}

	for _, np := range nps {
		if np.ContainerdConfig == "" {
			continue
		}
		for _, n := range np.Nodes {
			// Same host names as in the inventories.
			host := n.Name
			if np.GetDynamicNodePool() != nil {
				host = strings.TrimPrefix(n.Name, clusterID+"-")
			}
			if err := os.WriteFile(filepath.Join(dir, host+".toml"), []byte(np.ContainerdConfig), 0o600); err != nil {
				return fmt.Errorf("failed to write the containerd config of node %s: %w", n.Name, err)
			}
		}
	}
	return nil
}
