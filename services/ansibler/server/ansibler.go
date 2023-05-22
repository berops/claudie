package main

import (
	"fmt"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb"
)

const (
	baseDirectory   = "services/ansibler/server"
	inventoryFile   = "inventory.ini"
	outputDirectory = "clusters"
	privateKeyExt   = "pem"
)

type NodepoolInfo struct {
	Nodepools  []*pb.NodePool
	PrivateKey string
	ID         string
	Network    string
}

type AllNodesInventoryData struct {
	NodepoolInfos []*NodepoolInfo
}

type LbInventoryData struct {
	K8sNodepools []*pb.NodePool
	LBClusters   []*pb.LBcluster
	ClusterID    string
}

func generateInventoryFile(inventoryTemplate string, directory string, data interface{}) error {
	tpl, err := templateUtils.LoadTemplate(inventoryTemplate)
	if err != nil {
		return fmt.Errorf("error while loading template for %s : %w", directory, err)
	}
	template := templateUtils.Templates{Directory: directory}
	err = template.Generate(tpl, inventoryFile, data)
	if err != nil {
		return fmt.Errorf("error while generating from template for %s : %w", directory, err)
	}
	return nil
}
