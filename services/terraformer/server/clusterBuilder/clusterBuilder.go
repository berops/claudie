package clusterBuilder

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/backend"
	"github.com/berops/claudie/services/terraformer/server/provider"
	"github.com/berops/claudie/services/terraformer/server/terraform"
)

const (
	Output        = "services/terraformer/server/clusters"
	subnetCidrKey = "VPC_SUBNET_CIDR"
	// <nodepool-name>-subnet-cidr
	subnetCidrKeyTemplate = "%s-subnet-cidr"
	baseSubnetCIDR        = "10.0.0.0/24"
	defaultOctetToChange  = 2
)

// ClusterBuilder wraps data needed for building a cluster.
type ClusterBuilder struct {
	// DesiredInfo contains the information about the
	// desired state of the cluster.
	DesiredInfo *pb.ClusterInfo
	// CurrentInfo contains the information about the
	// current state of the cluster.
	CurrentInfo *pb.ClusterInfo
	// ProjectName is the name of the manifest.
	ProjectName string
	// ClusterType is the type of the cluster being build
	// LoadBalancer or K8s.
	ClusterType pb.ClusterType
	// Metadata contains data that further describe
	// the cluster that is to be build. For example,
	// in the case of LoadBalancer this will contain the defined
	// roles from the manifest. Can be nil if no data is supplied.
	Metadata map[string]any
}

type NodepoolsData struct {
	ClusterName string
	ClusterHash string
	NodePools   []*pb.NodePool
	Metadata    map[string]any
	Regions     []string
}

type outputNodepools struct {
	IPs map[string]interface{} `json:"-"`
}

func (c ClusterBuilder) CreateNodepools() error {
	clusterID := fmt.Sprintf("%s-%s", c.DesiredInfo.Name, c.DesiredInfo.Hash)
	clusterDir := filepath.Join(Output, clusterID)

	// Calculate CIDR, so they do not change if nodepool order changes
	// https://github.com/berops/claudie/issues/647
	// Order them by provider and region
	for _, nps := range utils.GroupNodepoolsByProviderRegion(c.DesiredInfo) {
		if err := c.calculateCIDR(baseSubnetCIDR, nps); err != nil {
			return fmt.Errorf("error while generating CIDR for nodepools : %w", err)
		}
	}

	if err := c.generateFiles(clusterID, clusterDir); err != nil {
		return fmt.Errorf("failed to generate files: %w", err)
	}

	terraform := terraform.Terraform{
		Directory: clusterDir,
	}

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		terraform.Stdout = comm.GetStdOut(clusterID)
		terraform.Stderr = comm.GetStdErr(clusterID)
	}

	if err := terraform.TerraformInit(); err != nil {
		return fmt.Errorf("error while running terraform init in %s : %w", clusterID, err)
	}

	if err := terraform.TerraformApply(); err != nil {
		return fmt.Errorf("error while running terraform apply in %s : %w", clusterID, err)
	}
	oldNodes := c.getCurrentNodes()

	// fill new nodes with output
	for _, nodepool := range c.DesiredInfo.NodePools {
		output, err := terraform.TerraformOutput(nodepool.Name)
		if err != nil {
			return fmt.Errorf("error while getting output from terraform for %s : %w", nodepool.Name, err)
		}
		out, err := readIPs(output)
		if err != nil {
			return fmt.Errorf("error while reading the terraform output for %s : %w", nodepool.Name, err)
		}
		fillNodes(&out, nodepool, oldNodes)
	}

	// Clean after terraform
	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("error while deleting files in %s : %w", clusterDir, err)
	}

	return nil
}

func (c ClusterBuilder) DestroyNodepools() error {
	clusterID := fmt.Sprintf("%s-%s", c.CurrentInfo.Name, c.CurrentInfo.Hash)
	clusterDir := filepath.Join(Output, clusterID)

	// Calculate CIDR, in case some nodepools do not have it, due to error.
	// https://github.com/berops/claudie/issues/647
	// Order them by provider and region
	for _, nps := range utils.GroupNodepoolsByProviderRegion(c.CurrentInfo) {
		if err := c.calculateCIDR(baseSubnetCIDR, nps); err != nil {
			return fmt.Errorf("error while generating CIDR for nodepools : %w", err)
		}
	}

	if err := c.generateFiles(clusterID, clusterDir); err != nil {
		return fmt.Errorf("failed to generate files: %w", err)
	}

	terraform := terraform.Terraform{
		Directory: clusterDir,
	}

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		terraform.Stdout = comm.GetStdOut(clusterID)
		terraform.Stderr = comm.GetStdErr(clusterID)
	}

	if err := terraform.TerraformInit(); err != nil {
		return fmt.Errorf("error while running terraform init in %s : %w", clusterID, err)
	}

	if err := terraform.TerraformDestroy(); err != nil {
		return fmt.Errorf("error while running terraform apply in %s : %w", clusterID, err)
	}

	// Clean after terraform.
	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("error while deleting files in %s : %w", clusterDir, err)
	}

	return nil
}

func (c *ClusterBuilder) generateFiles(clusterID, clusterDir string) error {
	backend := backend.Backend{
		ProjectName: c.ProjectName,
		ClusterName: clusterID,
		Directory:   clusterDir,
	}

	if err := backend.CreateFiles(); err != nil {
		return err
	}

	// generate Providers terraform configuration
	providers := provider.Provider{
		ProjectName: c.ProjectName,
		ClusterName: clusterID,
		Directory:   clusterDir,
	}

	if err := providers.CreateProvider(c.CurrentInfo, c.DesiredInfo); err != nil {
		return err
	}

	var clusterInfo *pb.ClusterInfo
	if c.DesiredInfo != nil {
		clusterInfo = c.DesiredInfo
	} else if c.CurrentInfo != nil {
		clusterInfo = c.CurrentInfo
	}

	// Init node slices if needed
	for _, np := range clusterInfo.NodePools {
		nodes := make([]*pb.Node, 0, np.Count)
		nodeNames := make(map[string]struct{}, np.Count)
		// Copy existing nodes into new slice
		for i, node := range np.Nodes {
			if i == int(np.Count) {
				break
			}
			log.Debug().Str("cluster", clusterID).Msgf("Nodepool is reusing node %s", node.Name)
			nodes = append(nodes, node)
			nodeNames[node.Name] = struct{}{}
		}
		// Fill the rest of the nodes with assigned names
		nodepoolID := fmt.Sprintf("%s-%s", clusterID, np.Name)
		for len(nodes) < int(np.Count) {
			// Get a unique name for the new node
			nodeName := getUniqueNodeName(nodepoolID, nodeNames)
			nodeNames[nodeName] = struct{}{}
			nodes = append(nodes, &pb.Node{Name: nodeName})
		}
		np.Nodes = nodes
	}

	suffix := getTplFile(c.ClusterType)
	// generate providers.tpl for all nodepools (current, desired).
	if err := generateProviderTemplates(c.CurrentInfo, c.DesiredInfo, clusterID, clusterDir, suffix); err != nil {
		return fmt.Errorf("error while generating provider templates: %w", err)
	}

	// sort nodepools by a provider
	sortedNodePools := utils.GroupNodepoolsByProviderSpecName(clusterInfo)
	for providerSpecName, nodepools := range sortedNodePools {
		providerName := nodepools[0].Provider.CloudProviderName

		// based on the cluster type fill out the nodepools data to be used
		nodepoolData := NodepoolsData{
			NodePools:   nodepools,
			ClusterName: clusterInfo.Name,
			ClusterHash: clusterInfo.Hash,
			Metadata:    c.Metadata,
			Regions:     utils.GetRegions(nodepools),
		}

		// Copy subnets CIDR to metadata
		copyCIDRsToMetadata(&nodepoolData)

		// Load TF files of the specific cloud provider
		targetDirectory := templateUtils.Templates{Directory: clusterDir}
		sourceDirectory := templateUtils.TemplateLoader{Directory: path.Join(templateUtils.TerraformerTemplates, providerName)}

		//  Generate the infra templates.
		tpl, err := sourceDirectory.LoadTemplate(fmt.Sprintf("%s%s", providerName, suffix))
		if err != nil {
			return fmt.Errorf("error while parsing template file %s : %w", fmt.Sprintf("%s%s", providerName, suffix), err)
		}

		if err := targetDirectory.Generate(tpl, fmt.Sprintf("%s-%s.tf", clusterID, providerSpecName), nodepoolData); err != nil {
			return fmt.Errorf("error while generating %s file : %w", fmt.Sprintf("%s-%s.tf", clusterID, providerSpecName), err)
		}

		// Create publicKey file for a cluster
		if err := utils.CreateKeyFile(clusterInfo.PublicKey, clusterDir, "public.pem"); err != nil {
			return fmt.Errorf("error creating key file for %s : %w", clusterDir, err)
		}

		// save keys
		if err := utils.CreateKeyFile(nodepools[0].Provider.Credentials, clusterDir, providerSpecName); err != nil {
			return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", providerSpecName, clusterDir, err)
		}
	}

	return nil
}

// getCurrentNodes returns all nodes which are in a current state
func (c *ClusterBuilder) getCurrentNodes() []*pb.Node {
	// group all the nodes together to make searching with respect to IP easy
	var oldNodes []*pb.Node
	if c.CurrentInfo != nil {
		for _, oldNodepool := range c.CurrentInfo.NodePools {
			oldNodes = append(oldNodes, oldNodepool.Nodes...)
		}
	}
	return oldNodes
}

// calculateCIDR will make sure all nodepools have subnet CIDR calculated.
func (c *ClusterBuilder) calculateCIDR(baseCIDR string, nodepools []*pb.NodePool) error {
	exists := make(map[string]struct{})
	// Save CIDRs which already exist.
	for _, np := range nodepools {
		if cidr, ok := np.Metadata[subnetCidrKey]; ok {
			exists[cidr.GetCidr()] = struct{}{}
		}
	}
	// Calculate new ones if needed.
	for _, np := range nodepools {
		// Check if CIDR key is defined and if value is not nil/empty.
		if cidr, ok := np.Metadata[subnetCidrKey]; !ok || cidr == nil || cidr.GetCidr() == "" {
			cidr, err := getCIDR(baseCIDR, defaultOctetToChange, exists)
			if err != nil {
				return fmt.Errorf("failed to parse CIDR for nodepool %s : %w", np.Name, err)
			}
			log.Debug().Msgf("Calculating new VPC subnet CIDR for nodepool. New CIDR [%s]", cidr)
			if np.Metadata == nil {
				np.Metadata = make(map[string]*pb.MetaValue)
			}
			np.Metadata[subnetCidrKey] = &pb.MetaValue{MetaValueOneOf: &pb.MetaValue_Cidr{Cidr: cidr}}
			// Cache calculated CIDR.
			exists[cidr] = struct{}{}
		}
	}
	return nil
}

// fillNodes creates pb.Node slices in desired state, with the new nodes and old nodes
func fillNodes(terraformOutput *outputNodepools, newNodePool *pb.NodePool, oldNodes []*pb.Node) {
	// fill slices from terraformOutput maps with names of nodes to ensure an order
	var tempNodes []*pb.Node
	// get sorted list of keys
	sortedNodeNames := getKeysFromMap(terraformOutput.IPs)
	for _, nodeName := range sortedNodeNames {
		var nodeType pb.NodeType
		var private string

		if newNodePool.IsControl {
			nodeType = pb.NodeType_master
		} else {
			nodeType = pb.NodeType_worker
		}
		if len(oldNodes) > 0 {
			for _, node := range oldNodes {
				//check if node was defined before
				if fmt.Sprint(terraformOutput.IPs[nodeName]) == node.Public && nodeName == node.Name {
					// carry privateIP to desired state, so it will not get overwritten in Ansibler
					private = node.Private
					// carry node type since it might be API endpoint, which should not change once set
					nodeType = node.NodeType
					break
				}
			}
		}

		tempNodes = append(tempNodes, &pb.Node{
			Name:     nodeName,
			Public:   fmt.Sprint(terraformOutput.IPs[nodeName]),
			Private:  private,
			NodeType: nodeType,
		})
	}
	newNodePool.Nodes = tempNodes
}

// getKeysFromMap returns an array of all keys in a map
func getKeysFromMap(data map[string]interface{}) []string {
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

// getUniqueNodeName returns new node name, which is guaranteed to be unique, based on the provided existing names.
func getUniqueNodeName(nodepoolID string, existingNames map[string]struct{}) string {
	index := 1
	for {
		candidate := fmt.Sprintf("%s-%d", nodepoolID, index)
		if _, ok := existingNames[candidate]; !ok {
			return candidate
		}
		index++
	}
}

// getCIDR function returns CIDR in IPv4 format, with position replaced by value
// The function does not check if it is a valid CIDR/can be used in subnet spec
func getCIDR(baseCIDR string, position int, existing map[string]struct{}) (string, error) {
	_, ipNet, err := net.ParseCIDR(baseCIDR)
	if err != nil {
		return "", fmt.Errorf("cannot parse a CIDR with base %s, position %d", baseCIDR, position)
	}
	ip := ipNet.IP
	ones, _ := ipNet.Mask.Size()
	var i int
	for {
		if i > 255 {
			return "", fmt.Errorf("maximum number of IPs assigned")
		}
		ip[position] = byte(i)
		if _, ok := existing[fmt.Sprintf("%s/%d", ip.String(), ones)]; ok {
			// CIDR already assigned.
			i++
			continue
		}
		// CIDR does not exist yet, return.
		return fmt.Sprintf("%s/%d", ip.String(), ones), nil
	}
}

// copyCIDRsToMetadata copies CIDRs for subnets in VPCs for every nodepool.
func copyCIDRsToMetadata(data *NodepoolsData) {
	if data.Metadata == nil {
		data.Metadata = make(map[string]any)
	}
	for _, np := range data.NodePools {
		data.Metadata[fmt.Sprintf(subnetCidrKeyTemplate, np.Name)] = np.Metadata[subnetCidrKey].GetCidr()
	}
}

// generateProviderTemplates generates only the `provider.tpl` templates so terraform can
// destroy the infra if needed.
func generateProviderTemplates(current, desired *pb.ClusterInfo, clusterID, directory, suffix string) error {
	currentNodepools := utils.GroupNodepoolsByProvider(current)
	desiredNodepools := utils.GroupNodepoolsByProvider(desired)

	// merge together into a single map instead of creating a new.
	for name, np := range desiredNodepools {
		if cnp, ok := currentNodepools[name]; !ok {
			currentNodepools[name] = np
		} else {
			// merge them together as different regions could be used.
			// (regions are used for generating the providers for various regions)
			for _, pool := range np {
				if found := utils.GetNodePoolByName(pool.Name, cnp); found == nil {
					currentNodepools[name] = append(currentNodepools[name], pool)
				}
			}
		}
	}

	info := desired
	if info == nil {
		info = current
	}

	for providerName, np := range currentNodepools {
		providerSpecName := np[0].Provider.SpecName

		nodepoolData := NodepoolsData{
			NodePools:   np,
			ClusterName: info.Name,
			ClusterHash: info.Hash,
			Metadata:    nil, // not needed
			Regions:     utils.GetRegions(np),
		}

		// Load TF files of the specific cloud provider
		sourceDirectory := templateUtils.TemplateLoader{
			Directory: path.Join(templateUtils.TerraformerTemplates, providerName),
		}

		targetDirectory := templateUtils.Templates{Directory: directory}

		tpl, err := sourceDirectory.LoadTemplate(fmt.Sprintf("%s-provider%s", providerName, suffix))
		if err != nil {
			return fmt.Errorf("error while parsing template file %s : %w", fmt.Sprintf("%s-provider%s", providerName, suffix), err)
		}

		// Parse the templates and create Tf files
		if err := targetDirectory.Generate(tpl, fmt.Sprintf("%s-%s-provider.tf", clusterID, providerSpecName), nodepoolData); err != nil {
			return fmt.Errorf("error while generating %s file : %w", fmt.Sprintf("%s-%s.tf", clusterID, providerSpecName), err)
		}

		// save keys
		if err = utils.CreateKeyFile(np[0].Provider.Credentials, directory, providerSpecName); err != nil {
			return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", providerSpecName, directory, err)
		}
	}

	return nil
}
