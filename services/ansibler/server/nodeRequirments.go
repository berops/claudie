package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/services/ansibler/server/ansible"
)

const (
	longhornReq = "../../ansible-playbooks/longhorn-req.yml"
)

func installLonghornRequirements(k8sNodepools []*NodepoolInfo) error {
	//since all nodes will be processed, fabricate dummy directory name with hash
	idHash := utils.CreateHash(5)
	directory := filepath.Join(baseDirectory, outputDirectory, idHash)
	//generate private key files
	for _, nodepoolInfo := range k8sNodepools {
		if _, err := os.Stat(directory); os.IsNotExist(err) {
			if err := os.MkdirAll(directory, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create dir: %v", err)
			}
		}
		if err := utils.CreateKeyFile(nodepoolInfo.PrivateKey, directory, fmt.Sprintf("%s.%s", nodepoolInfo.ID, privateKeyExt)); err != nil {
			return fmt.Errorf("failed to create key file: %v", err)
		}
	}
	//generate inventory file
	err := generateInventoryFile(nodesInventoryFileTpl, directory, AllNodesInventoryData{NodepoolInfos: k8sNodepools})
	if err != nil {
		return fmt.Errorf("failed to generate inventory file for all nodes : %v", err)
	}
	//run playbook
	ansible := ansible.Ansible{Playbook: longhornReq, Inventory: inventoryFile, Directory: directory}
	err = ansible.RunAnsiblePlaybook(fmt.Sprintf("ALL - %s", directory))
	if err != nil {
		return fmt.Errorf("error while running ansible to install Longhorn requirements : %v", err)
	}
	//Clean up
	if err := os.RemoveAll(directory); err != nil {
		return fmt.Errorf("error while deleting files: %v", err)
	}
	return nil
}
