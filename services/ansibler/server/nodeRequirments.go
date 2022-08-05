package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Berops/platform/services/ansibler/server/ansible"
	"github.com/Berops/platform/utils"
)

const (
	longhornReq     = "../../ansible-playbooks/longhorn-req.yml"
	outputDirectory = "clusters"
	privateKeyExt   = "pem"
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
	err := generateInventoryFile(nodesInventoryFile, directory, AllNodesInventoryData{NodepoolInfos: k8sNodepools})
	if err != nil {
		return err
	}
	//run playbook
	ansible := ansible.Ansible{Playbook: longhornReq, Inventory: inventoryFile, Directory: directory}
	err = ansible.RunAnsiblePlaybook(fmt.Sprintf("ALL NODES - %s", directory))
	if err != nil {
		return err
	}
	//Clean up
	if err := os.RemoveAll(directory); err != nil {
		return fmt.Errorf("error while deleting files: %v", err)
	}
	return nil
}
