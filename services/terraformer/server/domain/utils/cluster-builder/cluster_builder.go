package cluster_builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates/backend"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates/provider"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates/templates"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/terraform"
)

const (
	TemplatesRootDir = "services/terraformer/templates"
	Output           = "services/terraformer/server/clusters"
	subnetCidrKey    = "VPC_SUBNET_CIDR"
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

type TemplateGeneration struct {
	// Name of the Input Manifest.
	ProjectName string
	// Directory where the templates are to be generated.
	TargetDirectory string
	// Nodepool from which templates will be generated.
	Nodepool *pb.NodePool
	// Cluster relevant data.
	ClusterData      templates.ClusterData
	ClusterMetadata  map[string]any
	ClusterPublicKey string
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

	oldNodes := c.getCurrentNodes()
	// TODO: parralelize.
	for _, np := range c.DesiredClusterInfo.GetNodePools() {
		if np.GetDynamicNodePool() == nil {
			continue
		}

		g := TemplateGeneration{
			ProjectName:     c.ProjectName,
			TargetDirectory: filepath.Join(clusterDir, np.Name),
			Nodepool:        np,
			ClusterData: templates.ClusterData{
				ClusterName: c.DesiredClusterInfo.Name,
				ClusterHash: c.DesiredClusterInfo.Hash,
				ClusterType: c.ClusterType.String(),
			},
			ClusterMetadata:  c.Metadata,
			ClusterPublicKey: c.DesiredClusterInfo.PublicKey,
		}

		if err := generateFiles(np, &g); err != nil {
			return fmt.Errorf("failed to generate files: %w", err)
		}

		terraform := terraform.Terraform{
			Directory:         g.TargetDirectory,
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

		output, err := terraform.Output(np.Name)
		if err != nil {
			return fmt.Errorf("error while getting output from terraform for %s : %w", np.Name, err)
		}

		out, err := readIPs(output)
		if err != nil {
			return fmt.Errorf("error while reading the terraform output for %s : %w", np.Name, err)
		}

		fillNodes(&out, np, oldNodes)
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

	for _, np := range c.CurrentClusterInfo.GetNodePools() {
		if np.GetDynamicNodePool() == nil {
			continue
		}

		g := TemplateGeneration{
			ProjectName:     c.ProjectName,
			TargetDirectory: filepath.Join(clusterID, np.Name),
			Nodepool:        np,
			ClusterData: templates.ClusterData{
				ClusterName: c.CurrentClusterInfo.Name,
				ClusterHash: c.CurrentClusterInfo.Hash,
				ClusterType: c.ClusterType.String(),
			},
			ClusterMetadata:  c.Metadata,
			ClusterPublicKey: c.CurrentClusterInfo.PublicKey,
		}

		if err := generateFiles(np, &g); err != nil {
			return fmt.Errorf("failed to generate fiels: %w", err)
		}
		terraform := terraform.Terraform{
			Directory:         g.TargetDirectory,
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
	}

	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("error while deleting files in %s : %w", clusterDir, err)
	}

	return nil
}

// generateFiles creates all the necessary terraform files used to create/destroy node pools.
func generateFiles(np *pb.NodePool, tg *TemplateGeneration) error {
	clusterID := fmt.Sprintf("%s-%s", tg.ClusterData.ClusterName, tg.ClusterData.ClusterHash)

	b := backend.Backend{
		Key:       fmt.Sprintf("%s/%s/%s", tg.ProjectName, clusterID, np.Name),
		Directory: tg.TargetDirectory,
	}

	if err := backend.Create(&b); err != nil {
		return err
	}

	if err := provider.CreateNodepool(tg.TargetDirectory, np); err != nil {
		return err
	}

	updateNodes(np, clusterID)

	// TODO: to this with the DNS aswell (i.e. error handling EMPTY error)
	repo := templates.Repository{TemplatesRootDirectory: TemplatesRootDir}
	if err := repo.Download(np.GetDynamicNodePool().GetTemplates()); err != nil {
		if errors.Is(err, templates.EmptyRepositoryErr) {
			return fmt.Errorf("nodepool %q does not have a template repository: %w", np.Name, err)
		}
		return err
	}

	if err := generateProviderTemplates(tg.TargetDirectory, clusterID, np, tg.ClusterData); err != nil {
		return fmt.Errorf("error while generating provider templates: %w", err)
	}

	g := templates.NodepoolGenerator{
		ClusterID:         clusterID,
		Nodepool:          np,
		TargetDirectory:   tg.TargetDirectory,
		ReadFromDirectory: TemplatesRootDir,
	}

	providerData := templates.ProviderData{
		ClusterData: tg.ClusterData,
		Provider:    np.GetDynamicNodePool().GetProvider(),
		Region:      np.GetDynamicNodePool().GetRegion(),
		Metadata:    tg.ClusterMetadata,
	}
	if err := g.GenerateNetworkingCommon(&providerData); err != nil {
		return fmt.Errorf("failed to generate networking_common template files: %w", err)
	}

	if err := utils.CreateKeyFile(dnp.PublicKey, clusterDir, fmt.Sprintf("%s.pem", np.Name)); err != nil {
		return fmt.Errorf("error public key file for %s : %w", clusterDir, err)
	}

	// based on the cluster type fill out the nodepools data to be used
	nodepoolData := templates.NodepoolsData{
		ClusterData: tg.ClusterData,
		NodePool: templates.NodePoolInfo{
			Name:      np.Name,
			Nodes:     np.Nodes,
			NodePool:  np.GetDynamicNodePool(),
			IsControl: np.IsControl,
		},
		Metadata: tg.ClusterMetadata,
	}
	copyCIDRsToMetadata(&nodepoolData)

	if err := g.GenerateNodes(&nodepoolData, &providerData); err != nil {
		return fmt.Errorf("failed to generate nodepool specific templates files: %w", err)
	}

	err := utils.CreateKeyFile(
		np.GetDynamicNodePool().Provider.GetCredentials(),
		tg.TargetDirectory,
		np.GetDynamicNodePool().Provider.GetSpecName(),
	)
	if err != nil {
		return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", np.GetDynamicNodePool().Provider.GetSpecName(), tg.TargetDirectory, err)
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
func fillNodes(terraformOutput *templates.NodepoolIPs, newNodePool *pb.NodePool, oldNodes []*pb.Node) {
	// fill slices from terraformOutput maps with names of nodes to ensure an order
	var tempNodes []*pb.Node
	// get sorted list of keys
	utils.IterateInOrder(terraformOutput.IPs, func(nodeName string, IP any) error {
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
				if fmt.Sprint(IP) == node.Public && nodeName == node.Name {
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
		return nil
	})

	newNodePool.Nodes = tempNodes
}

// readIPs reads json output format from terraform and unmarshal it into map[string]map[string]string readable by Go.
func readIPs(data string) (templates.NodepoolIPs, error) {
	var result templates.NodepoolIPs
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
func copyCIDRsToMetadata(data *templates.NodepoolsData) {
	if data.Metadata == nil {
		data.Metadata = make(map[string]any)
	}
	key := fmt.Sprintf(subnetCidrKeyTemplate, data.NodePool.Name)
	data.Metadata[key] = data.NodePool.NodePool.Metadata[subnetCidrKey].GetCidr()
}

// generateProviderTemplates generates only the `provider.tpl` templates so terraform can
// destroy the infra if needed.
func generateProviderTemplates(targetDirectory, clusterID string, np *pb.NodePool, clusterData templates.ClusterData) error {
	g := templates.NodepoolGenerator{
		ClusterID:         clusterID,
		Nodepool:          np,
		TargetDirectory:   targetDirectory,
		ReadFromDirectory: TemplatesRootDir,
	}

	err := g.GenerateProvider(&templates.ProviderData{
		ClusterData: clusterData,
		Provider:    np.GetDynamicNodePool().GetProvider(),
		Region:      np.GetDynamicNodePool().GetRegion(),
		Metadata:    nil, // not needed.
	})

	if err != nil {
		return fmt.Errorf("failed to generate provider templates: %w", err)
	}
	return nil
}

func updateNodes(np *pb.NodePool, clusterID string) {
	n := np.GetDynamicNodePool()

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
