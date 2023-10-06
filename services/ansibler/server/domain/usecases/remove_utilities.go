package usecases

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"path/filepath"

	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
)

func (u *Usecases) RemoveUtilities(req *pb.RemoveClaudieUtilitiesRequest) (*pb.RemoveClaudieUtilitiesResponse, error) {
	logger := cutils.CreateLoggerWithProjectAndClusterName(req.ProjectName, req.Current.ClusterInfo.Name)
	logger.Info().Msgf("Removing Claudie installed utilities")

	vpnInfo := &VPNInfo{
		ClusterNetwork: req.Current.Network,
		NodepoolsInfos: []*NodepoolsInfo{
			{
				Nodepools: utils.NodePools{
					Dynamic: cutils.GetCommonDynamicNodePools(req.Current.ClusterInfo.NodePools),
					Static:  cutils.GetCommonStaticNodePools(req.Current.ClusterInfo.NodePools),
				},
				PrivateKey:     req.Current.ClusterInfo.PrivateKey,
				ClusterID:      cutils.GetClusterID(req.Current.ClusterInfo),
				ClusterNetwork: req.Current.Network,
			},
		},
	}

	for _, lbCluster := range req.CurrentLbs {
		vpnInfo.NodepoolsInfos = append(vpnInfo.NodepoolsInfos, &NodepoolsInfo{
			Nodepools: utils.NodePools{
				Dynamic: cutils.GetCommonDynamicNodePools(lbCluster.ClusterInfo.NodePools),
				Static:  cutils.GetCommonStaticNodePools(lbCluster.ClusterInfo.NodePools),
			},
			PrivateKey:     lbCluster.ClusterInfo.PrivateKey,
			ClusterID:      cutils.GetClusterID(lbCluster.ClusterInfo),
			ClusterNetwork: req.Current.Network,
		})
	}

	if err := removeWireguard(cutils.GetClusterID(req.Current.ClusterInfo), vpnInfo, u.SpawnProcessLimit); err != nil {
		return nil, fmt.Errorf("failed to remove wiregaurd from nodes: %w", err)
	}

	return &pb.RemoveClaudieUtilitiesResponse{Current: req.Current, CurrentLbs: req.CurrentLbs}, nil
}

func removeWireguard(clusterID string, vpnInfo *VPNInfo, spawnProcessLimit chan struct{}) error {
	clusterDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", clusterID, cutils.CreateHash(cutils.HashLength)))
	if err := cutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", clusterDirectory, err)
	}

	err := utils.GenerateInventoryFile(templates.AllNodesInventoryTemplate, clusterDirectory, AllNodesInventoryData{
		NodepoolsInfo: vpnInfo.NodepoolsInfos,
	})
	if err != nil {
		return fmt.Errorf("error while creating inventory file for %s: %w", clusterDirectory, err)
	}

	for _, nodepoolInfo := range vpnInfo.NodepoolsInfos {
		if err := cutils.CreateKeyFile(nodepoolInfo.PrivateKey, clusterDirectory, fmt.Sprintf("%s.%s", nodepoolInfo.ClusterID, sshPrivateKeyFileExtension)); err != nil {
			return fmt.Errorf("failed to create key file for %s : %w", nodepoolInfo.ClusterID, err)
		}

		if err := cutils.CreateKeysForStaticNodepools(nodepoolInfo.Nodepools.Static, clusterDirectory); err != nil {
			return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
		}
	}

	ansible := utils.Ansible{
		Playbook:          wireguardUninstall,
		Inventory:         utils.InventoryFileName,
		Directory:         clusterDirectory,
		SpawnProcessLimit: spawnProcessLimit,
	}

	// Subsequent calling may fail, thus simply log the error.
	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Remove Utils - %s", clusterID)); err != nil {
		log.Warn().Msgf("error while uninstalling wireguard ansible for %s : %w", clusterID, err)
	}

	return os.RemoveAll(clusterDirectory)
}
