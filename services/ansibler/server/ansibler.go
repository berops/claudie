package main

import (
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
)

const (
	baseDirectory      = "services/ansibler/server"
	inventoryFile      = "inventory.ini"
	nodesInventoryFile = "all-node-inventory.goini"
)

type NodepoolInfo struct {
	Nodepools  []*pb.NodePool
	PrivateKey string
	ID         string
}

type AllNodesInventoryData struct {
	NodepoolInfos []*NodepoolInfo
}

type LbInventoryData struct {
	K8sNodepools []*pb.NodePool
	LBClusters   []*pb.LBcluster
}

func generateInventoryFile(inventoryTemplate, directory string, data interface{}) error {
	templateLoader := utils.TemplateLoader{Directory: utils.AnsiblerTemplates}
	tpl, err := templateLoader.LoadTemplate(inventoryTemplate)
	if err != nil {
		return err
	}
	template := utils.Templates{Directory: directory}
	err = template.Generate(tpl, inventoryFile, data)
	if err != nil {
		return err
	}
	return nil
}
