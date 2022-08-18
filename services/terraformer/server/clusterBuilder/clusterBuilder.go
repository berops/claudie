package clusterBuilder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/terraformer/server/backend"
	"github.com/Berops/platform/services/terraformer/server/provider"
	"github.com/Berops/platform/services/terraformer/server/terraform"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
)

// desiredInfo - clusterInfo of desired state, currentInfo - clusterInfo of current state
type ClusterBuilder struct {
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

type outputNodepools struct {
	IPs map[string]interface{} `json:"-"`
}

const (
	Output            = "services/terraformer/server/clusters"
	TemplateDirectory = "services/terraformer/templates"
)

// tplFile - template file for creation of nodepools
func (c ClusterBuilder) CreateNodepools() error {
	clusterID := fmt.Sprintf("%s-%s", c.DesiredInfo.Name, c.DesiredInfo.Hash)
	clusterDir := filepath.Join(Output, clusterID)
	fmt.Println(clusterDir)
	terraform := terraform.Terraform{Directory: clusterDir, StdOut: utils.GetStdOut(clusterID), StdErr: utils.GetStdErr(clusterID)}
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

	// get slice of old nodes
	oldNodes := c.getNodes()

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

	// Clean after terraform
	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("error while deleting files: %v", err)
	}

	return nil
}

// tplFile - template file for creation of nodepools
func (c ClusterBuilder) DestroyNodepools() error {
	clusterID := fmt.Sprintf("%s-%s", c.CurrentInfo.Name, c.CurrentInfo.Hash)
	clusterDir := filepath.Join(Output, clusterID)
	terraform := terraform.Terraform{Directory: clusterDir, StdOut: utils.GetStdOut(clusterID), StdErr: utils.GetStdErr(clusterID)}
	//generate template files
	err := c.generateFiles(clusterID, clusterDir)
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
	// Clean after terraform
	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("error while deleting files: %v", err)
	}
	return nil
}

func (c ClusterBuilder) generateFiles(clusterID, clusterDir string) error {
	// generate backend
	backend := backend.Backend{ProjectName: c.ProjectName, ClusterName: clusterID, Directory: clusterDir}
	err := backend.CreateFiles()
	if err != nil {
		return err
	}

	// generate .tf files for nodepools
	var clusterInfo *pb.ClusterInfo
	template := utils.Templates{Directory: clusterDir}
	templateLoader := utils.TemplateLoader{Directory: utils.TerraformerTemplates}

	if c.DesiredInfo != nil {
		clusterInfo = c.DesiredInfo
	} else if c.CurrentInfo != nil {
		clusterInfo = c.CurrentInfo
	}

	// generate Providers terraform configuration
	providers := provider.Provider{ProjectName: c.ProjectName, ClusterName: clusterID, Directory: clusterDir}
	err = providers.CreateProvider(clusterInfo)
	if err != nil {
		return err
	}

	tplType := getTplFile(c.ClusterType)
	//sort nodepools by a provider
	sortedNodePools := utils.GroupNodepoolsByProviderSpecName(clusterInfo)
	for providerSpecName, nodepools := range sortedNodePools {
		nodepoolData := NodepoolsData{
			NodePools:   nodepools,
			ClusterName: clusterInfo.Name,
			ClusterHash: clusterInfo.Hash,
		}

		// Load TF files of the specific cloud provider
		tpl, err := templateLoader.LoadTemplate(fmt.Sprintf("%s%s", nodepools[0].Provider.CloudProviderName, tplType))
		if err != nil {
			return fmt.Errorf("error while parsing template file backend.tpl: %v", err)
		}

		// Parse the templates and create Tf files
		err = template.Generate(tpl, fmt.Sprintf("%s-%s.tf", clusterID, providerSpecName), nodepoolData)
		if err != nil {
			return fmt.Errorf("error while generating .tf files : %v", err)
		}

		// Create publicKey file for a cluster
		if err := utils.CreateKeyFile(clusterInfo.PublicKey, clusterDir, "public.pem"); err != nil {
			log.Error().Msgf("Error creating key file: %v", err)
			return err
		}

		// save keys
		if err = utils.CreateKeyFile(nodepools[0].Provider.Credentials, clusterDir, providerSpecName); err != nil {
			log.Error().Msgf("Error creating provider credential key file: %v", err)
			return err
		}

	}

	return nil
}

func (c ClusterBuilder) getNodes() []*pb.Node {
	// group all the nodes together to make searching with respect to IP easy
	var oldNodes []*pb.Node
	if c.CurrentInfo != nil {
		for _, oldNodepool := range c.CurrentInfo.NodePools {
			oldNodes = append(oldNodes, oldNodepool.Nodes...)
		}
	}
	return oldNodes
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
	switch clusterType {
	case pb.ClusterType_K8s:
		return ".tpl"
	case pb.ClusterType_LB:
		return "-lb.tpl"
	}
	return ""
}
