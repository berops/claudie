package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/services/ansibler/server/ansible"
	"github.com/berops/claudie/services/ansibler/templates"
)

const (
	longhornReq = "../../ansible-playbooks/longhorn-req.yml"
)

func installLonghornRequirements(nodepoolInfo *NodepoolInfo) error {
	//since all nodes will be processed, fabricate dummy directory name with hash
	directory := filepath.Join(baseDirectory, outputDirectory, utils.CreateHash(utils.HashLength))

	if err := utils.CreateDirectory(directory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", directory, err)
	}

	//generate private key files
	if err := utils.CreateKeyFile(nodepoolInfo.PrivateKey, directory, fmt.Sprintf("%s.%s", nodepoolInfo.ID, privateKeyExt)); err != nil {
		return fmt.Errorf("failed to create key file for %s : %w", nodepoolInfo.ID, err)
	}

	for _, snp := range nodepoolInfo.Nodepools.Static {
		for ep, key := range snp.NodeKeys {
			if err := utils.CreateKeyFile(key, directory, fmt.Sprintf("%s.%s", getNodeName(snp, ep), privateKeyExt)); err != nil {
				return fmt.Errorf("failed to create key file for : %w", err)
			}
		}
	}

	if err := generateInventoryFile(templates.AllNodesInventoryTemplate, directory, AllNodesInventoryData{NodepoolInfos: []*NodepoolInfo{nodepoolInfo}}); err != nil {
		return fmt.Errorf("failed to generate inventory file for all nodes in %s : %w", directory, err)
	}

	ansible := ansible.Ansible{Playbook: longhornReq, Inventory: inventoryFile, Directory: directory}
	if err := ansible.RunAnsiblePlaybook(fmt.Sprintf("ALL - %s", nodepoolInfo.ID)); err != nil {
		return fmt.Errorf("error while running ansible to install Longhorn requirements for %s : %w", directory, err)
	}

	return os.RemoveAll(directory)
}
