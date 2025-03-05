package usecases

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

func (u *Usecases) RemoveUtilities(req *pb.RemoveClaudieUtilitiesRequest) (*pb.RemoveClaudieUtilitiesResponse, error) {
	logger := loggerutils.WithProjectAndCluster(req.ProjectName, req.Current.ClusterInfo.Id())
	logger.Info().Msgf("Removing Claudie installed utilities")

	vpnInfo := &VPNInfo{
		ClusterNetwork: req.Current.Network,
		NodepoolsInfos: []*NodepoolsInfo{
			{
				Nodepools: utils.NodePools{
					Dynamic: nodepools.Dynamic(req.Current.ClusterInfo.NodePools),
					Static:  nodepools.Static(req.Current.ClusterInfo.NodePools),
				},
				ClusterID:      req.Current.ClusterInfo.Id(),
				ClusterNetwork: req.Current.Network,
			},
		},
	}

	for _, lbCluster := range req.CurrentLbs {
		vpnInfo.NodepoolsInfos = append(vpnInfo.NodepoolsInfos, &NodepoolsInfo{
			Nodepools: utils.NodePools{
				Dynamic: nodepools.Dynamic(lbCluster.ClusterInfo.NodePools),
				Static:  nodepools.Static(lbCluster.ClusterInfo.NodePools),
			},
			ClusterID:      lbCluster.ClusterInfo.Id(),
			ClusterNetwork: req.Current.Network,
		})
	}

	if err := removeUtilities(req.Current.ClusterInfo.Id(), vpnInfo, u.SpawnProcessLimit); err != nil {
		return nil, fmt.Errorf("failed to remove wiregaurd from nodes: %w", err)
	}

	return &pb.RemoveClaudieUtilitiesResponse{}, nil
}

func removeUtilities(clusterID string, vpnInfo *VPNInfo, processLimit *semaphore.Weighted) error {
	clusterDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, hash.Create(hash.Length)))
	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", clusterDirectory, err)
	}

	err := utils.GenerateInventoryFile(templates.AllNodesInventoryTemplate, clusterDirectory, AllNodesInventoryData{
		NodepoolsInfo: vpnInfo.NodepoolsInfos,
	})
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
		RetryCount:        3,
		Playbook:          wireguardUninstall,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: processLimit,
	}

	// Subsequent calling may fail, thus simply log the error.
	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Remove Utils - %s", clusterID)); err != nil {
		log.Warn().Msgf("error while uninstalling wireguard ansible for %s : %s", clusterID, err)
	}

	return os.RemoveAll(clusterDirectory)
}
