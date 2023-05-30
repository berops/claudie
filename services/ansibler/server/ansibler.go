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
	Nodepools  NodePools
	PrivateKey string
	ID         string
	Network    string
}

type AllNodesInventoryData struct {
	NodepoolInfos []*NodepoolInfo
}

type LbInventoryData struct {
	K8sNodepools NodePools
	LBClusters   []*LBcluster
	ClusterID    string
}

type NodePools struct {
	Dynamic []*pb.DynamicNodePool
	Static  []*pb.StaticNodePool
}

type LBcluster struct {
	Name        string
	Hash        string
	LBnodepools NodePools
}

func generateInventoryFile(inventoryTemplate string, directory string, data interface{}) error {
	tpl, err := templateUtils.LoadTemplate(inventoryTemplate)
	if err != nil {
		return fmt.Errorf("error while loading inventory template for %s : %w", directory, err)
	}
	template := templateUtils.Templates{Directory: directory}
	err = template.Generate(tpl, inventoryFile, data)
	if err != nil {
		return fmt.Errorf("error while generating from inventory template for %s : %w", directory, err)
	}
	return nil
}

// getNodeName returns name of a static node which uses specified endpoint.
func getNodeName(snp *pb.StaticNodePool, ep string) string {
	for _, node := range snp.Nodes {
		if node.Public == ep {
			return node.Name
		}
	}
	return ""
}
