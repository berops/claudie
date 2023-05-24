package usecases

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/utils"
)

const ansiblePlaybookFilePath = "../../ansible-playbooks/longhorn-req.yml"

// InstallNodeRequirements installs pre-requisite tools (currently only for LongHorn) on all the nodes
func (u *Usecases) InstallNodeRequirements(request *pb.InstallRequest) (*pb.InstallResponse, error) {
	logger := log.With().Str("project", request.ProjectName).Str("cluster", request.Desired.ClusterInfo.Name).Logger()
	logger.Info().Msgf("Installing node requirements")

	nodepoolsInfoOfCluster := &NodepoolsInfoOfCluster{
		Nodepools:      request.Desired.ClusterInfo.NodePools,
		PrivateKey:     request.Desired.ClusterInfo.PrivateKey,
		ClusterId:      fmt.Sprintf("%s-%s", request.Desired.ClusterInfo.Name, request.Desired.ClusterInfo.Hash),
		ClusterNetwork: request.Desired.Network,
	}

	if err := installLonghornRequirements(nodepoolsInfoOfCluster); err != nil {
		logger.Err(err).Msgf("Error encountered while installing node requirements")
		return nil, fmt.Errorf("error encountered while installing node requirements for cluster %s project %s : %w", request.Desired.ClusterInfo.Name, request.ProjectName, err)
	}

	logger.Info().Msgf("Node requirements were successfully installed")
	return &pb.InstallResponse{}, nil
}

// installLonghornRequirements installs pre-requisite tools for LongHorn in all the nodes
func installLonghornRequirements(nodepoolsInfo *NodepoolsInfoOfCluster) error {
	// Directory where files (required by Ansible) will be generated.
	outputDirectory := filepath.Join(baseDirectory, outputDirectory, commonUtils.CreateHash(commonUtils.HashLength))
	if err := commonUtils.CreateDirectory(outputDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", outputDirectory, err)
	}

	// generate private SSH key which will be used by Ansible
	if err := commonUtils.CreateKeyFile(nodepoolsInfo.PrivateKey, outputDirectory, fmt.Sprintf("%s.%s", nodepoolsInfo.ClusterId, sshPrivateKeyFileExtension)); err != nil {
		return fmt.Errorf("failed to create key file for %s : %w", nodepoolsInfo.ClusterId, err)
	}

	if err := utils.GenerateInventoryFile(allNodesInventoryTemplateFileName, outputDirectory,
		// Value of Ansible template parameters
		AllNodesInventoryData{
			NodepoolsInfos: []*NodepoolsInfoOfCluster{nodepoolsInfo},
		},
	); err != nil {
		return fmt.Errorf("failed to generate inventory file for all nodes in %s : %w", outputDirectory, err)
	}

	ansible := utils.Ansible{
		Playbook:  ansiblePlaybookFilePath,
		Inventory: inventoryFileName,
		Directory: outputDirectory,
	}
	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("Node requirements - %s", nodepoolsInfo.ClusterId)); err != nil {
		return fmt.Errorf("error while running ansible playbook at %s to install Longhorn requirements : %w", outputDirectory, err)
	}

	return os.RemoveAll(outputDirectory)
}
