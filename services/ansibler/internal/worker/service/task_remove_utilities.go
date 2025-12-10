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

const wireguardUninstall = "../../ansible-playbooks/wireguard-uninstall.yml"

// RemoveUtilities removes claudie installed utilities.
func RemoveUtilities(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Removing Claudie utilities")

	delete, ok := tracker.Task.Do.(*spec.TaskV2_Delete)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to remove claudie utilities, assuming task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	k8s := delete.Delete.K8S
	lbs := delete.Delete.LoadBalancers

	vpnInfo := VPNInfo{
		ClusterNetwork: k8s.Network,
		NodepoolsInfos: []*NodepoolsInfo{
			{
				Nodepools: utils.NodePools{
					Dynamic: nodepools.Dynamic(k8s.ClusterInfo.NodePools),
					Static:  nodepools.Static(k8s.ClusterInfo.NodePools),
				},
				ClusterID:      k8s.ClusterInfo.Id(),
				ClusterNetwork: k8s.Network,
			},
		},
	}

	for _, lb := range lbs {
		vpnInfo.NodepoolsInfos = append(vpnInfo.NodepoolsInfos, &NodepoolsInfo{
			Nodepools: utils.NodePools{
				Dynamic: nodepools.Dynamic(lb.ClusterInfo.NodePools),
				Static:  nodepools.Static(lb.ClusterInfo.NodePools),
			},
			ClusterID:      lb.ClusterInfo.Id(),
			ClusterNetwork: k8s.Network,
		})
	}

	logger.Info().Msgf("Starting cleanup of utilities installed by Claudie, may take a while")

	if err := removeUtilities(k8s.ClusterInfo.Id(), &vpnInfo, processLimit); err != nil {
		logger.Err(err).Msg("Failed to remove claudie utilities from nodes of the cluster")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Successfully removed claudie utilities")
}

func removeUtilities(clusterID string, vpnInfo *VPNInfo, processLimit *semaphore.Weighted) error {
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
			NodepoolsInfo: vpnInfo.NodepoolsInfos,
		},
	)
	if err != nil {
		return fmt.Errorf("error while creating inventory file for %s: %w", clusterDirectory, err)
	}

	for _, nodepoolInfo := range vpnInfo.NodepoolsInfos {
		if err := nodepools.DynamicGenerateKeys(nodepoolInfo.Nodepools.Dynamic, clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
		}

		if err := nodepools.StaticGenerateKeys(nodepoolInfo.Nodepools.Static, clusterDirectory); err != nil {
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
