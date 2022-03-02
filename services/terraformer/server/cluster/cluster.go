package cluster

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/terraformer/server/templates"
	"github.com/Berops/platform/services/terraformer/server/terraform"
	"github.com/rs/zerolog/log"
)

// desiredInfo - clusterInfo of desired state, currentInfo - clusterInfo of current state
type Cluster struct {
	DesiredInfo *pb.ClusterInfo
	CurrentInfo *pb.ClusterInfo
	ProjectName string
	ClusterType pb.ClusterType
}

type NodepoolsData struct {
	NodePools   []*pb.NodePool
	ClusterName string
	ClusterHash string
}

type BackendData struct {
	ProjectName string
	ClusterName string
}

type outputNodepools struct {
	IPs map[string]interface{} `json:"-"`
}

const (
	output = "services/terraformer/server/clusters"
)

// tplFile - template file for creation of nodepools
func (c Cluster) CreateNodepools() error {
	clusterID := fmt.Sprintf(c.DesiredInfo.Name, "-", c.DesiredInfo.Hash)
	clusterDir := filepath.Join(output, clusterID)
	terraform := terraform.Terraform{Directory: clusterDir}
	err := c.generateFiles(clusterID, clusterDir)
	if err != nil {
		// description of an error in c.generateFiles()
		return err
	}
	// create nodepool with terraform
	err = terraform.TerraformInit()
	if err != nil {
		return fmt.Errorf("error while running terraform init in %s : %v", clusterID, err)
	}
	err = terraform.TerraformApply()
	if err != nil {
		return fmt.Errorf("error while running terraform apply in %s : %v", clusterID, err)
	}

	// group all the nodes together to make searching with respect to IP easy
	var oldNodes []*pb.Node
	if c.CurrentInfo != nil {
		for _, oldNodepool := range c.CurrentInfo.NodePools {
			oldNodes = append(oldNodes, oldNodepool.Nodes...)
		}
	}

	// fill new nodes with output
	for _, nodepool := range c.DesiredInfo.NodePools {
		output, err := terraform.TerraformOutput(nodepool.Name)
		if err != nil {
			log.Error().Msgf("Error while getting output from terraform: %v", err)
			return err
		}
		out, err := readIPs(output)
		if err != nil {
			log.Error().Msgf("Error while reading the terraform output: %v", err)
			return err
		}
		fillNodes(&out, nodepool, oldNodes)
	}

	return nil
}

// tplFile - template file for creation of nodepools
func (c Cluster) DestroyNodepools(tplFile string) error {
	clusterID := fmt.Sprintf(c.DesiredInfo.Name, "-", c.DesiredInfo.Hash)
	clusterDir := filepath.Join(output, clusterID)
	terraform := terraform.Terraform{Directory: clusterDir}
	//generate template files
	err := c.generateFiles(clusterID, clusterDir, tplFile)
	if err != nil {
		// description of an error in c.generateFiles()
		return err
	}
	// destroy nodepools with terraform
	err = terraform.TerraformInit()
	if err != nil {
		return fmt.Errorf("error while running terraform init in %s : %v", clusterID, err)
	}
	err = terraform.TerraformDestroy()
	if err != nil {
		return fmt.Errorf("error while running terraform apply in %s : %v", clusterID, err)
	}
	return nil
}

func (c Cluster) generateFiles(clusterID, clusterDir string) error {
	// generate .tf files from templates
	backend := BackendData{
		ProjectName: c.ProjectName,
		ClusterName: clusterID,
	}
	nodepools := NodepoolsData{
		NodePools:   c.DesiredInfo.NodePools,
		ClusterName: c.DesiredInfo.Name,
		ClusterHash: c.DesiredInfo.Hash,
	}
	templates := templates.Templates{Directory: clusterDir}
	err := templates.Generate("backend.tpl", "backend.tf", backend)
	if err != nil {
		return fmt.Errorf("error while generating backend.tf for %s : %v", clusterID, err)
	}
	tplFile := getTplFile(c.ClusterType)
	err = templates.Generate(tplFile, fmt.Sprintf("%s.tf", clusterID), nodepools)
	if err != nil {
		return fmt.Errorf("error while generating .tf files for %s : %v", clusterID, err)
	}
	return nil
}

func fillNodes(terraformOutput *outputNodepools, newNodePool *pb.NodePool, oldNodes []*pb.Node) {
	// Fill slices from terraformOutput maps with names of nodes to ensure an order
	var tempNodes []*pb.Node

	// get sorted list of keys
	sortedNodeNames := getkeysFromMap(terraformOutput.IPs)
	for _, nodeName := range sortedNodeNames {
		var private = ""
		var control pb.NodeType

		if newNodePool.IsControl {
			control = pb.NodeType_master
		} else {
			control = pb.NodeType_worker
		}

		if len(oldNodes) > 0 {
			for _, node := range oldNodes {
				if fmt.Sprint(terraformOutput.IPs[nodeName]) == node.Public {
					private = node.Private
					control = node.NodeType
					break
				}
			}
		}
		tempNodes = append(tempNodes, &pb.Node{
			Name:     nodeName,
			Public:   fmt.Sprint(terraformOutput.IPs[nodeName]),
			Private:  private,
			NodeType: control,
		})
	}
	newNodePool.Nodes = tempNodes
}

// getKeysFromMap returns an array of all keys in a map
func getkeysFromMap(data map[string]interface{}) []string {
	var keys []string
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// readIPs reads json output format from terraform and unmarshal it into map[string]map[string]string readable by GO
func readIPs(data string) (outputNodepools, error) {
	var result outputNodepools
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result.IPs)
	return result, err
}

func getTplFile(clusterType pb.ClusterType) string {

}
