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
	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned terraform
	// processes. This values should always be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
}

type ClusterData struct {
	ClusterName string
	ClusterHash string
	ClusterType string
}

type ProviderData struct {
	ClusterData ClusterData
	Provider    *pb.Provider
	Regions     []string
	Metadata    map[string]any
}

type NodepoolsData struct {
	ClusterData ClusterData
	NodePools   []NodePoolInfo
	Metadata    map[string]any
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
	clusterID := utils.GetClusterID(c.DesiredClusterInfo)
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
		Directory:         clusterDir,
		SpawnProcessLimit: c.SpawnProcessLimit,
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
	clusterID := utils.GetClusterID(c.CurrentClusterInfo)
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
		Directory:         clusterDir,
		SpawnProcessLimit: c.SpawnProcessLimit,
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

	if err := generateProviderTemplates(c.CurrentClusterInfo, c.DesiredClusterInfo, clusterID, clusterDir); err != nil {
		return fmt.Errorf("error while generating provider templates: %w", err)
	}

	groupedNodepools := utils.GroupNodepoolsByProviderNames(clusterInfo)
	for providerNames, nodepools := range groupedNodepools {
		if providerNames.CloudProviderName == pb.StaticNodepoolInfo_STATIC_PROVIDER.String() {
			continue
		}

		providerData := &ProviderData{
			ClusterData: ClusterData{
				ClusterName: clusterInfo.Name,
				ClusterHash: clusterInfo.Hash,
				ClusterType: c.ClusterType.String(),
			},
			Provider: nodepools[0].GetDynamicNodePool().GetProvider(),
			Regions:  utils.GetRegions(utils.GetDynamicNodePools(nodepools)),
			Metadata: c.Metadata,
		}

		if err := generateNetworkingCommon(clusterID, clusterDir, providerData); err != nil {
			return fmt.Errorf("failed to generate networking_common template files: %w", err)
		}

		nps := make([]NodePoolInfo, 0, len(nodepools))
		for _, np := range nodepools {
			if np.GetDynamicNodePool() == nil {
				continue
			}
			nps = append(nps, NodePoolInfo{
				Name:      np.Name,
				Nodes:     np.Nodes,
				NodePool:  np.GetDynamicNodePool(),
				IsControl: np.IsControl,
			})
		}

		// based on the cluster type fill out the nodepools data to be used
		nodepoolData := NodepoolsData{
			ClusterData: ClusterData{
				ClusterName: clusterInfo.Name,
				ClusterHash: clusterInfo.Hash,
				ClusterType: c.ClusterType.String(),
			},
			NodePools: nps,
			Metadata:  c.Metadata,
		}

		copyCIDRsToMetadata(&nodepoolData)

		if err := generateNodes(clusterID, clusterDir, &nodepoolData, providerData); err != nil {
			return fmt.Errorf("failed to generate nodepool specific templates files: %w", err)
		}

		if err := utils.CreateKeyFile(clusterInfo.PublicKey, clusterDir, "public.pem"); err != nil {
			return fmt.Errorf("error creating key file for %s : %w", clusterDir, err)
		}

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

// getUniqueNodeName returns new node name, which is guaranteed to be unique, based on the provided existing names.
func getUniqueNodeName(nodepoolID string, existingNames map[string]struct{}) string {
	index := uint8(1)
	for {
		candidate := fmt.Sprintf("%s-%02x", nodepoolID, index)
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
func generateProviderTemplates(current, desired *pb.ClusterInfo, clusterID, directory string) error {
	currentNodepools := utils.GroupNodepoolsByProviderNames(current)
	desiredNodepools := utils.GroupNodepoolsByProviderNames(desired)

	// merge together into a single map instead of creating a new.
	for name, np := range desiredNodepools {
		// Continue if static node pool provider.
		if name.CloudProviderName == pb.StaticNodepoolInfo_STATIC_PROVIDER.String() {
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
		if providerName.CloudProviderName == pb.StaticNodepoolInfo_STATIC_PROVIDER.String() {
			continue
		}

		providerData := ProviderData{
			ClusterData: ClusterData{
				ClusterName: info.Name,
				ClusterHash: info.Hash,
				ClusterType: "", // not needed.
			},
			Provider: nodepools[0].GetDynamicNodePool().GetProvider(),
			Regions:  utils.GetRegions(utils.GetDynamicNodePools(nodepools)),
			Metadata: nil, // not needed.
		}

		if err := generateProvider(clusterID, directory, &providerData); err != nil {
			return fmt.Errorf("failed to generate provider templates: %w", err)
		}
	}

	return nil
}

func generateProvider(clusterID, directory string, data *ProviderData) error {
	targetDirectory := templateUtils.Templates{Directory: directory}

	tplPath := filepath.Join(data.Provider.CloudProviderName, "provider.tpl")

	file, err := templates.CloudProviderTemplates.ReadFile(tplPath)
	if err != nil {
		return fmt.Errorf("error while reading template file %s : %w", tplPath, err)
	}

	tpl, err := templateUtils.LoadTemplate(string(file))
	if err != nil {
		return fmt.Errorf("error while parsing template file %s : %w", tplPath, err)
	}

	providerSpecName := data.Provider.SpecName

	// Parse the templates and create Tf files
	if err := targetDirectory.Generate(tpl, fmt.Sprintf("%s-%s-provider.tf", clusterID, providerSpecName), data); err != nil {
		return fmt.Errorf("error while generating %s file : %w", fmt.Sprintf("%s-%s-provider.tf", clusterID, providerSpecName), err)
	}

	// Save keys
	if err = utils.CreateKeyFile(data.Provider.Credentials, directory, providerSpecName); err != nil {
		return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", providerSpecName, directory, err)
	}

	return nil
}

func generateNetworkingCommon(clusterID, directory string, data *ProviderData) error {
	var (
		targetDirectory = templateUtils.Templates{Directory: directory}
		tplPath         = filepath.Join(data.Provider.CloudProviderName, "networking.tpl")
		provider        = data.Provider.CloudProviderName
		specName        = data.Provider.SpecName
	)

	file, err := templates.CloudProviderTemplates.ReadFile(tplPath)
	if err != nil {
		return fmt.Errorf("error while reading networking template file %s: %w", provider, err)
	}

	networking, err := templateUtils.LoadTemplate(string(file))
	if err != nil {
		return fmt.Errorf("error while parsing networking_common template file %s : %w", provider, err)
	}

	err = targetDirectory.Generate(networking, fmt.Sprintf("%s-%s-networkingn.tf", clusterID, specName), data)
	if err != nil {
		return fmt.Errorf("error while generating %s file : %w", fmt.Sprintf("%s-%s-networking.tf", clusterID, specName), err)
	}

	return nil
}

func generateNodes(clusterID, directory string, data *NodepoolsData, providerData *ProviderData) error {
	var (
		targetDirectory = templateUtils.Templates{Directory: directory}
		networkingPath  = filepath.Join(providerData.Provider.CloudProviderName, "node_networking.tpl")
		nodesPath       = filepath.Join(providerData.Provider.CloudProviderName, "node.tpl")
		provider        = providerData.Provider.CloudProviderName
		specName        = providerData.Provider.SpecName
	)

	file, err := templates.CloudProviderTemplates.ReadFile(networkingPath)
	if err == nil { // the template file might not exists
		networking, err := templateUtils.LoadTemplate(string(file))
		if err != nil {
			return fmt.Errorf("error while parsing node networking template file %s : %w", provider, err)
		}
		if err := targetDirectory.Generate(networking, fmt.Sprintf("%s-%s-node-networking.tf", clusterID, specName), data); err != nil {
			return fmt.Errorf("error while generating %s file : %w", fmt.Sprintf("%s-%s-node-networking.tf", clusterID, specName), err)
		}
	}

	file, err = templates.CloudProviderTemplates.ReadFile(nodesPath)
	if err != nil {
		return fmt.Errorf("error while reading nodepool template file %s: %w", provider, err)
	}

	nodepool, err := templateUtils.LoadTemplate(string(file))
	if err != nil {
		return fmt.Errorf("error while parsing nodepool template file %s: %w", provider, err)
	}

	err = targetDirectory.Generate(nodepool, fmt.Sprintf("%s-%s-nodepool.tf", clusterID, specName), data)
	if err != nil {
		return fmt.Errorf("error while generating %s file: %w", fmt.Sprintf("%s-%s.tf", clusterID, specName), err)
	}

	return nil
}
