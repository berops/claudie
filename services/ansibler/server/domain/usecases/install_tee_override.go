package usecases

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/semaphore"
)

const ansibleTeeOverridePlaybookFilePath = "../../ansible-playbooks/tee-override.yml"

// InstallTeeOverride installs tee binary override on nodes
func (u *Usecases) InstallTeeOverride(request *pb.InstallTeeOverrideRequest) (*pb.InstallTeeOverrideResponse, error) {
	logger := log.With().Str("project", request.ProjectName).Str("cluster", request.Desired.ClusterInfo.Name).Logger()
	logger.Info().Msgf("Installing tee binary override")

	NodepoolsInfo := &NodepoolsInfo{
		Nodepools: utils.NodePools{
			Dynamic: nodepools.Dynamic(request.Desired.ClusterInfo.NodePools),
			Static:  nodepools.Static(request.Desired.ClusterInfo.NodePools),
		},
		ClusterID:      request.Desired.ClusterInfo.Id(),
		ClusterNetwork: request.Desired.Network,
	}

	if err := installTeeOverride(NodepoolsInfo, u.SpawnProcessLimit); err != nil {
		logger.Err(err).Msgf("Error encountered while installing tee binary override")
		return nil, fmt.Errorf("error encountered while installing tee binary override for cluster %s project %s : %w", request.Desired.ClusterInfo.Name, request.ProjectName, err)
	}

	logger.Info().Msgf("Tee binary override was successfully installed")
	return &pb.InstallTeeOverrideResponse{}, nil
}

// installTeeOverride injects the tee binary override on all the nodes
func installTeeOverride(nodepoolsInfo *NodepoolsInfo, processLimit *semaphore.Weighted) error {
	// Directory where files (required by Ansible) will be generated.
	clusterDirectory := filepath.Join(baseDirectory, outputDirectory, hash.Create(hash.Length))
	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	if err := nodepools.DynamicGenerateKeys(nodepoolsInfo.Nodepools.Dynamic, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools: %w", err)
	}

	if err := nodepools.StaticGenerateKeys(nodepoolsInfo.Nodepools.Static, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	if err := utils.GenerateInventoryFile(templates.AllNodesInventoryTemplate, clusterDirectory,
		// Value of Ansible template parameters
		AllNodesInventoryData{
			NodepoolsInfo: []*NodepoolsInfo{nodepoolsInfo},
		},
	); err != nil {
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

	return os.RemoveAll(clusterDirectory)
}
