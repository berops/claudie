package cluster_builder

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/backend"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/provider"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/terraform"
	"github.com/berops/claudie/services/terraformer/templates"
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
	// DesiredClusterInfo contains the information about the
	// desired state of the cluster.
	DesiredClusterInfo *pb.ClusterInfo
	// CurrentClusterInfo contains the information about the
	// current state of the cluster.
	CurrentClusterInfo *pb.ClusterInfo
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
	NodePools   []NodePoolInfo
	Metadata    map[string]any
	Regions     []string
}

type NodePoolInfo struct {
	NodePool  *pb.DynamicNodePool
	Name      string
	Nodes     []*pb.Node
	IsControl bool
}

type outputNodepools struct {
	IPs map[string]interface{} `json:"-"`
}

// CreateNodepools creates node pools for the cluster.
func (c ClusterBuilder) CreateNodepools() error {
	clusterID := fmt.Sprintf("%s-%s", c.DesiredClusterInfo.Name, c.DesiredClusterInfo.Hash)
	clusterDir := filepath.Join(Output, clusterID)

	// Calculate CIDR, so they do not change if nodepool order changes
	// https://github.com/berops/claudie/issues/647
	// Order them by provider and region
	for _, nps := range utils.GroupNodepoolsByProviderRegion(c.DesiredClusterInfo) {
		if err := c.calculateCIDR(baseSubnetCIDR, utils.GetDynamicNodePools(nps)); err != nil {
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

	if err := terraform.Init(); err != nil {
		return fmt.Errorf("error while running terraform init in %s : %w", clusterID, err)
	}

	if err := terraform.Apply(); err != nil {
		return fmt.Errorf("error while running terraform apply in %s : %w", clusterID, err)
	}
	oldNodes := c.getCurrentNodes()

	// fill new nodes with output
	for _, nodepool := range c.DesiredClusterInfo.NodePools {
		if np := nodepool.GetDynamicNodePool(); np != nil {
			output, err := terraform.Output(nodepool.Name)
			if err != nil {
				return fmt.Errorf("error while getting output from terraform for %s : %w", nodepool.Name, err)
			}
			out, err := readIPs(output)
			if err != nil {
				return fmt.Errorf("error while reading the terraform output for %s : %w", nodepool.Name, err)
			}
			fillNodes(&out, nodepool, oldNodes)
		}
	}

	// Clean after terraform
	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("error while deleting files in %s : %w", clusterDir, err)
	}

	return nil
}

// DestroyNodepools destroys nodepools for the cluster.
func (c ClusterBuilder) DestroyNodepools() error {
	clusterID := fmt.Sprintf("%s-%s", c.CurrentClusterInfo.Name, c.CurrentClusterInfo.Hash)
	clusterDir := filepath.Join(Output, clusterID)

	// Calculate CIDR, in case some nodepools do not have it, due to error.
	// https://github.com/berops/claudie/issues/647
	// Order them by provider and region
	for _, nps := range utils.GroupNodepoolsByProviderRegion(c.CurrentClusterInfo) {
		if err := c.calculateCIDR(baseSubnetCIDR, utils.GetDynamicNodePools(nps)); err != nil {
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

	if err := terraform.Init(); err != nil {
		return fmt.Errorf("error while running terraform init in %s : %w", clusterID, err)
	}

	if err := terraform.Destroy(); err != nil {
		return fmt.Errorf("error while running terraform apply in %s : %w", clusterID, err)
	}

	// Clean after terraform.
	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("error while deleting files in %s : %w", clusterDir, err)
	}

	return nil
}

// generateFiles creates all the necessary terraform files used to create/destroy node pools.
func (c *ClusterBuilder) generateFiles(clusterID, clusterDir string) error {
	backend := backend.Backend{
		ProjectName: c.ProjectName,
		ClusterName: clusterID,
		Directory:   clusterDir,
	}

	if err := backend.CreateTFFile(); err != nil {
		return err
	}

	// generate Providers terraform configuration
	providers := provider.Provider{
		ProjectName: c.ProjectName,
		ClusterName: clusterID,
		Directory:   clusterDir,
	}

	if err := providers.CreateProvider(c.CurrentClusterInfo, c.DesiredClusterInfo); err != nil {
		return err
	}

	var clusterInfo *pb.ClusterInfo
	if c.DesiredClusterInfo != nil {
		clusterInfo = c.DesiredClusterInfo
	} else if c.CurrentClusterInfo != nil {
		clusterInfo = c.CurrentClusterInfo
	}

	// Init node slices if needed
	for _, np := range clusterInfo.NodePools {
		if n := np.GetDynamicNodePool(); n != nil {
			nodes := make([]*pb.Node, 0, n.Count)
			nodeNames := make(map[string]struct{}, n.Count)
			// Copy existing nodes into new slice
			for i, node := range np.Nodes {
				if i == int(n.Count) {
					break
				}
				log.Debug().Str("cluster", clusterID).Msgf("Nodepool is reusing node %s", node.Name)
				nodes = append(nodes, node)
				nodeNames[node.Name] = struct{}{}
			}
			// Fill the rest of the nodes with assigned names
			nodepoolID := fmt.Sprintf("%s-%s", clusterID, np.Name)
			for len(nodes) < int(n.Count) {
				// Get a unique name for the new node
				nodeName := getUniqueNodeName(nodepoolID, nodeNames)
				nodeNames[nodeName] = struct{}{}
				nodes = append(nodes, &pb.Node{Name: nodeName})
			}
			np.Nodes = nodes
		}
	}

	suffix := getTplFile(c.ClusterType)
	// generate providers.tpl for all nodepools (current, desired).
	if err := generateProviderTemplates(c.CurrentClusterInfo, c.DesiredClusterInfo, clusterID, clusterDir, suffix); err != nil {
		return fmt.Errorf("error while generating provider templates: %w", err)
	}

	// sort nodepools by a provider
	sortedNodePools := utils.GroupNodepoolsByProviderNames(clusterInfo)
	for providerNames, nodepools := range sortedNodePools {
		providerName := providerNames.CloudProviderName
		// Continue if static node pool provider.
		if providerName == pb.StaticProvider_STATIC_PROVIDER.String() {
			continue
		}

		nps := make([]NodePoolInfo, 0, len(nodepools))
		for _, np := range nodepools {
			if np.GetDynamicNodePool() == nil {
				continue
			}
			nps = append(nps, NodePoolInfo{
				Name:     np.Name,
				Nodes:    np.Nodes,
				NodePool: np.GetDynamicNodePool(),
			})
		}

		// based on the cluster type fill out the nodepools data to be used
		nodepoolData := NodepoolsData{
			NodePools:   nps,
			ClusterName: clusterInfo.Name,
			ClusterHash: clusterInfo.Hash,
			Metadata:    c.Metadata,
			Regions:     utils.GetRegions(utils.GetDynamicNodePools(nodepools)),
		}

		// Copy subnets CIDR to metadata
		copyCIDRsToMetadata(&nodepoolData)

		// Load TF files of the specific cloud provider
		targetDirectory := templateUtils.Templates{Directory: clusterDir}

		//  Generate the infra templates.
		file, err := templates.CloudProviderTemplates.ReadFile(filepath.Join(providerName, suffix))
		if err != nil {
			return fmt.Errorf("error while reading template file %s : %w", fmt.Sprintf("%s/%s", providerName, suffix), err)
		}
		tpl, err := templateUtils.LoadTemplate(string(file))
		if err != nil {
			return fmt.Errorf("error while parsing template file %s : %w", fmt.Sprintf("%s/%s", providerName, suffix), err)
		}

		if err := targetDirectory.Generate(tpl, fmt.Sprintf("%s-%s.tf", clusterID, providerNames.SpecName), nodepoolData); err != nil {
			return fmt.Errorf("error while generating %s file : %w", fmt.Sprintf("%s-%s.tf", clusterID, providerNames.SpecName), err)
		}

		// Create publicKey file for a cluster
		if err := utils.CreateKeyFile(clusterInfo.PublicKey, clusterDir, "public.pem"); err != nil {
			return fmt.Errorf("error creating key file for %s : %w", clusterDir, err)
		}

		// save keys
		if err := utils.CreateKeyFile(nps[0].NodePool.Provider.Credentials, clusterDir, providerNames.SpecName); err != nil {
			return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", providerNames.SpecName, clusterDir, err)
		}
	}

	return nil
}

// getCurrentNodes returns all nodes which are in a current state
func (c *ClusterBuilder) getCurrentNodes() []*pb.Node {
	// group all the nodes together to make searching with respect to IP easy
	var oldNodes []*pb.Node
	if c.CurrentClusterInfo != nil {
		for _, oldNodepool := range c.CurrentClusterInfo.NodePools {
			if oldNodepool.GetDynamicNodePool() != nil {
				oldNodes = append(oldNodes, oldNodepool.Nodes...)
			}
		}
	}
	return oldNodes
}

// calculateCIDR will make sure all nodepools have subnet CIDR calculated.
func (c *ClusterBuilder) calculateCIDR(baseCIDR string, nodepools []*pb.DynamicNodePool) error {
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
				return fmt.Errorf("failed to parse CIDR for nodepool : %w", err)
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

// readIPs reads json output format from terraform and unmarshal it into map[string]map[string]string readable by Go.
func readIPs(data string) (outputNodepools, error) {
	var result outputNodepools
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result.IPs)
	return result, err
}

// getTplFile returns type of the template file.
func getTplFile(clusterType pb.ClusterType) string {
	switch clusterType {
	case pb.ClusterType_K8s:
		return "k8s.tpl"
	case pb.ClusterType_LB:
		return "lb.tpl"
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
		data.Metadata[fmt.Sprintf(subnetCidrKeyTemplate, np.Name)] = np.NodePool.Metadata[subnetCidrKey].GetCidr()
	}
}

// generateProviderTemplates generates only the `provider.tpl` templates so terraform can
// destroy the infra if needed.
func generateProviderTemplates(current, desired *pb.ClusterInfo, clusterID, directory, suffix string) error {
	currentNodepools := utils.GroupNodepoolsByProviderNames(current)
	desiredNodepools := utils.GroupNodepoolsByProviderNames(desired)

	// merge together into a single map instead of creating a new.
	for name, np := range desiredNodepools {
		// Continue if static node pool provider.
		if name.CloudProviderName == pb.StaticProvider_STATIC_PROVIDER.String() {
			continue
		}

		if cnp, ok := currentNodepools[name]; !ok {
			currentNodepools[name] = np
		} else {
			// merge them together as different regions could be used.
			// (regions are used for generating the providers for various regions)
			for _, pool := range np {
				if found := utils.GetNodePoolByName(pool.GetName(), cnp); found == nil {
					currentNodepools[name] = append(currentNodepools[name], pool)
				}
			}
		}
	}

	info := desired
	if info == nil {
		info = current
	}

	for providerName, nodepools := range currentNodepools {
		// Continue if static node pool provider.
		if providerName.CloudProviderName == pb.StaticProvider_STATIC_PROVIDER.String() {
			continue
		}
		nps := make([]NodePoolInfo, 0, len(nodepools))
		for _, np := range nodepools {
			if np.GetDynamicNodePool() == nil {
				continue
			}
			nps = append(nps, NodePoolInfo{
				Name:     np.Name,
				Nodes:    np.Nodes,
				NodePool: np.GetDynamicNodePool(),
			})
		}

		providerSpecName := providerName.SpecName

		nodepoolData := NodepoolsData{
			NodePools:   nps,
			ClusterName: info.Name,
			ClusterHash: info.Hash,
			Metadata:    nil, // not needed
			Regions:     utils.GetRegions(utils.GetDynamicNodePools(nodepools)),
		}

		// Load TF files of the specific cloud provider
		targetDirectory := templateUtils.Templates{Directory: directory}
		tplPath := filepath.Join(providerName.CloudProviderName, fmt.Sprintf("provider-%s", suffix))
		file, err := templates.CloudProviderTemplates.ReadFile(tplPath)
		if err != nil {
			return fmt.Errorf("error while reading template file %s : %w", tplPath, err)
		}
		tpl, err := templateUtils.LoadTemplate(string(file))
		if err != nil {
			return fmt.Errorf("error while parsing template file %s : %w", tplPath, err)
		}

		// Parse the templates and create Tf files
		if err := targetDirectory.Generate(tpl, fmt.Sprintf("%s-%s-provider.tf", clusterID, providerSpecName), nodepoolData); err != nil {
			return fmt.Errorf("error while generating %s file : %w", fmt.Sprintf("%s-%s-provider.tf", clusterID, providerSpecName), err)
		}

		// Save keys
		if err = utils.CreateKeyFile(nps[0].NodePool.Provider.Credentials, directory, providerSpecName); err != nil {
			return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", providerSpecName, directory, err)
		}
	}

	return nil
}
