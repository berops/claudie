package cluster_builder

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net"
	"os"
	"path/filepath"
	"slices"

	"github.com/berops/claudie/internal/checksum"
	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/templates"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/terraform"
	"github.com/rs/zerolog/log"
)

const (
	TemplatesRootDir     = "services/terraformer/templates"
	Output               = "services/terraformer/server/clusters"
	baseSubnetCIDR       = "10.0.0.0/24"
	defaultOctetToChange = 2
)

type K8sInfo struct{ LoadBalancers []*spec.LBcluster }
type LBInfo struct{ Roles []*spec.Role }

// ClusterBuilder wraps data needed for building a cluster.
type ClusterBuilder struct {
	// DesiredClusterInfo contains the information about the
	// desired state of the cluster.
	DesiredClusterInfo *spec.ClusterInfo
	// CurrentClusterInfo contains the information about the
	// current state of the cluster.
	CurrentClusterInfo *spec.ClusterInfo
	// ProjectName is the name of the manifest.
	ProjectName string
	// ClusterType is the type of the cluster being build
	// LoadBalancer or K8s.
	ClusterType spec.ClusterType
	// K8sInfo contains additional data for when building kubernetes clusters.
	K8sInfo K8sInfo
	// LBInfo contains additional data for when building loadbalancer clusters.
	LBInfo LBInfo
	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned terraform
	// processes. This values should always be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
}

// CreateNodepools creates node pools for the cluster.
func (c ClusterBuilder) CreateNodepools() error {
	clusterID := utils.GetClusterID(c.DesiredClusterInfo)
	clusterDir := filepath.Join(Output, clusterID)

	defer func() {
		// Clean after terraform
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Err(err).Msgf("error while deleting files in %s : %v", clusterDir, err)
		}
	}()

	// Calculate CIDR, so they do not change if nodepool order changes
	// https://github.com/berops/claudie/issues/647
	// Order them by provider and region
	for _, nps := range utils.GroupNodepoolsByProviderRegion(c.DesiredClusterInfo) {
		if err := calculateCIDR(baseSubnetCIDR, utils.GetDynamicNodePools(nps)); err != nil {
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

	terraform.Stdout = comm.GetStdOut(clusterID)
	terraform.Stderr = comm.GetStdErr(clusterID)

	if err := terraform.Init(); err != nil {
		return fmt.Errorf("error while running terraform init in %s : %w", clusterID, err)
	}

	var currentState []string
	if c.CurrentClusterInfo != nil {
		var err error
		if currentState, err = terraform.StateList(); err != nil {
			return fmt.Errorf("error while running terraform state list in %s : %w", clusterID, err)
		}
	}

	if err := terraform.Apply(); err != nil {
		updatedState, listErr := terraform.StateList()
		if listErr != nil {
			return errors.Join(err, fmt.Errorf("error while running terraform state list in %s : %w", clusterID, listErr))
		}

		// TODO(fix): this does not consider dependencies among the resources created.
		var errAll error
		for _, resource := range updatedState {
			if !slices.Contains(currentState, resource) {
				log.Debug().Msgf("deleting unsuccessfuly build resource %s", resource)
				if err := terraform.DestroyTarget(resource); err != nil {
					errAll = errors.Join(errAll, fmt.Errorf("error while running terraform destroy target %s in %s : %w", resource, clusterID, err))
				}
			}
		}

		err = fmt.Errorf("error while running terraform apply in %s:%w", clusterID, err)
		if errAll != nil {
			err = fmt.Errorf("%w: %w", err, errAll)
		}
		return err
	}
	oldNodes := c.getCurrentNodes()

	// fill new nodes with output
	for _, nodepool := range c.DesiredClusterInfo.NodePools {
		np := nodepool.GetDynamicNodePool()
		if np == nil {
			continue
		}

		f := checksum.Digest128(filepath.Join(np.Provider.SpecName, templates.ExtractTargetPath(np.Provider.Templates)))
		k := fmt.Sprintf("%s_%s_%s", nodepool.Name, np.Provider.SpecName, hex.EncodeToString(f))

		output, err := terraform.Output(k)
		if err != nil {
			return fmt.Errorf("error while getting output from terraform for %s : %w", nodepool.Name, err)
		}
		out, err := readIPs(output)
		if err != nil {
			return fmt.Errorf("error while reading the terraform output for %s : %w", nodepool.Name, err)
		}
		fillNodes(&out, nodepool, oldNodes)
	}

	return nil
}

// DestroyNodepools destroys nodepools for the cluster.
func (c ClusterBuilder) DestroyNodepools() error {
	clusterID := utils.GetClusterID(c.CurrentClusterInfo)
	clusterDir := filepath.Join(Output, clusterID)

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			log.Err(err).Msgf("error while deleting files in %s : %v", clusterDir, err)
		}
	}()

	// Calculate CIDR, in case some nodepools do not have it, due to error.
	// https://github.com/berops/claudie/issues/647
	// Order them by provider and region
	for _, nps := range utils.GroupNodepoolsByProviderRegion(c.CurrentClusterInfo) {
		if err := calculateCIDR(baseSubnetCIDR, utils.GetDynamicNodePools(nps)); err != nil {
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

	terraform.Stdout = comm.GetStdOut(clusterID)
	terraform.Stderr = comm.GetStdErr(clusterID)

	if err := terraform.Init(); err != nil {
		return fmt.Errorf("error while running terraform init in %s : %w", clusterID, err)
	}

	if err := terraform.Destroy(); err != nil {
		return fmt.Errorf("error while running terraform apply in %s : %w", clusterID, err)
	}

	return nil
}

// generateFiles creates all the necessary terraform files used to create/destroy node pools.
func (c *ClusterBuilder) generateFiles(clusterID, clusterDir string) error {
	backend := templates.Backend{
		ProjectName: c.ProjectName,
		ClusterName: clusterID,
		Directory:   clusterDir,
	}

	if err := backend.CreateTFFile(); err != nil {
		return err
	}

	// generate Providers terraform configuration
	usedProviders := templates.UsedProviders{
		ProjectName: c.ProjectName,
		ClusterName: clusterID,
		Directory:   clusterDir,
	}

	if err := usedProviders.CreateUsedProvider(c.CurrentClusterInfo, c.DesiredClusterInfo); err != nil {
		return err
	}

	var clusterInfo *spec.ClusterInfo
	if c.DesiredClusterInfo != nil {
		clusterInfo = c.DesiredClusterInfo
	} else if c.CurrentClusterInfo != nil {
		clusterInfo = c.CurrentClusterInfo
	}

	clusterData := templates.ClusterData{
		ClusterName: clusterInfo.Name,
		ClusterHash: clusterInfo.Hash,
		ClusterType: c.ClusterType.String(),
	}

	if err := c.generateProviderTemplates(c.CurrentClusterInfo, c.DesiredClusterInfo, clusterID, clusterDir, clusterData); err != nil {
		return fmt.Errorf("error while generating provider templates: %w", err)
	}

	for info, pools := range GroupByProvider(clusterInfo.NodePools) {
		templatesDownloadDir := filepath.Join(TemplatesRootDir, clusterID, info.SpecName)

		for path, pools := range GroupByTemplates(pools) {
			p := pools[0].GetDynamicNodePool().GetProvider()

			if err := templates.DownloadProvider(templatesDownloadDir, p); err != nil {
				msg := fmt.Sprintf("cluster %q failed to download template repository", clusterID)
				log.Error().Msgf("%v", msg)
				return fmt.Errorf("%s: %w", msg, err)
			}

			nps := make([]templates.NodePoolInfo, 0, len(pools))

			for _, np := range pools {
				if dnp := np.GetDynamicNodePool(); dnp != nil {
					nps = append(nps, templates.NodePoolInfo{
						Name:      np.Name,
						Nodes:     np.Nodes,
						Details:   np.GetDynamicNodePool(),
						IsControl: np.IsControl,
					})

					if err := utils.CreateKeyFile(dnp.GetPublicKey(), clusterDir, np.GetName()); err != nil {
						return fmt.Errorf("error public key file for %s : %w", clusterDir, err)
					}
				}
			}

			// based on the cluster type fill out the nodepools data to be used
			nodepoolData := templates.Nodepools{
				ClusterData: clusterData,
				NodePools:   nps,
			}

			g := templates.Generator{
				ID:                clusterID,
				TargetDirectory:   clusterDir,
				ReadFromDirectory: templatesDownloadDir,
				TemplatePath:      path,
				Fingerprint:       hex.EncodeToString(checksum.Digest128(filepath.Join(info.SpecName, path))),
			}

			if err := g.GenerateNetworking(&templates.Networking{
				ClusterData: clusterData,
				Provider:    p,
				Regions:     utils.GetRegions(utils.GetDynamicNodePools(pools)),
				K8sData: templates.K8sData{
					HasAPIServer: !slices.Contains(
						utils.ExtractTargetPorts(c.K8sInfo.LoadBalancers),
						6443,
					),
				},
				LBData: templates.LBData{
					Roles: c.LBInfo.Roles,
				},
			}); err != nil {
				return fmt.Errorf("failed to generate networking_common template files: %w", err)
			}

			if err := g.GenerateNodes(&nodepoolData); err != nil {
				return fmt.Errorf("failed to generate nodepool specific templates files: %w", err)
			}
		}
	}

	return nil
}

// getCurrentNodes returns all nodes which are in a current state
func (c *ClusterBuilder) getCurrentNodes() []*spec.Node {
	// group all the nodes together to make searching with respect to IP easy
	var oldNodes []*spec.Node
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
func calculateCIDR(baseCIDR string, nodepools []*spec.DynamicNodePool) error {
	exists := make(map[string]struct{})
	// Save CIDRs which already exist.
	for _, np := range nodepools {
		exists[np.Cidr] = struct{}{}
	}

	// Calculate new ones if needed.
	for _, np := range nodepools {
		if np.Cidr != "" {
			continue
		}

		cidr, err := getCIDR(baseCIDR, defaultOctetToChange, exists)
		if err != nil {
			return fmt.Errorf("failed to parse CIDR for nodepool : %w", err)
		}

		log.Debug().Msgf("Calculating new VPC subnet CIDR for nodepool. New CIDR [%s]", cidr)
		np.Cidr = cidr
		// Cache calculated CIDR.
		exists[cidr] = struct{}{}
	}

	return nil
}

// fillNodes creates pb.Node slices in desired state, with the new nodes and old nodes
func fillNodes(terraformOutput *templates.NodepoolIPs, newNodePool *spec.NodePool, oldNodes []*spec.Node) {
	// fill slices from terraformOutput maps with names of nodes to ensure an order
	var tempNodes []*spec.Node
	// get sorted list of keys
	_ = utils.IterateInOrder(terraformOutput.IPs, func(nodeName string, IP any) error {
		var nodeType spec.NodeType
		var private string

		if newNodePool.IsControl {
			nodeType = spec.NodeType_master
		} else {
			nodeType = spec.NodeType_worker
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

		tempNodes = append(tempNodes, &spec.Node{
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

// generateProviderTemplates generates only the `provider.tpl` templates so terraform can destroy the infra if needed.
func (c *ClusterBuilder) generateProviderTemplates(current, desired *spec.ClusterInfo, clusterID, directory string, clusterData templates.ClusterData) error {
	nps := slices.AppendSeq(
		slices.Collect(slices.Values(current.GetNodePools())),
		slices.Values(desired.GetNodePools()),
	)

	for info, pools := range GroupByProvider(nps) {
		if err := utils.CreateKeyFile(info.Creds, directory, info.SpecName); err != nil {
			return fmt.Errorf("error creating provider credential key file for provider %s in %s : %w", info.SpecName, directory, err)
		}

		templatesDownloadDir := filepath.Join(TemplatesRootDir, clusterID, info.SpecName)

		for path, pools := range GroupByTemplates(pools) {
			p := pools[0].GetDynamicNodePool().GetProvider()
			if err := templates.DownloadProvider(templatesDownloadDir, p); err != nil {
				msg := fmt.Sprintf("cluster %q failed to download template repository", clusterID)
				log.Error().Msgf("%v", msg)
				return fmt.Errorf("%s: %w", msg, err)
			}

			g := templates.Generator{
				ID:                clusterID,
				TargetDirectory:   directory,
				ReadFromDirectory: templatesDownloadDir,
				TemplatePath:      path,
				Fingerprint:       hex.EncodeToString(checksum.Digest128(filepath.Join(info.SpecName, path))),
			}

			err := g.GenerateProvider(&templates.Provider{
				ClusterData: clusterData,
				Provider:    pools[0].GetDynamicNodePool().GetProvider(),
				Regions:     utils.GetRegions(utils.GetDynamicNodePools(pools)),
			})

			if err != nil {
				return fmt.Errorf("failed to generate provider templates: %w", err)
			}
		}
	}

	return nil
}

type ProviderTemplateGroup struct {
	CloudProvider string
	SpecName      string
	Creds         string
}

func GroupByProvider(nps []*spec.NodePool) iter.Seq2[ProviderTemplateGroup, []*spec.NodePool] {
	m := make(map[ProviderTemplateGroup][]*spec.NodePool)

	for _, nodepool := range nps {
		np, ok := nodepool.Type.(*spec.NodePool_DynamicNodePool)
		if !ok {
			continue
		}
		k := ProviderTemplateGroup{
			CloudProvider: np.DynamicNodePool.Provider.CloudProviderName,
			SpecName:      np.DynamicNodePool.Provider.SpecName,
			Creds:         utils.GetAuthCredentials(np.DynamicNodePool.Provider),
		}
		m[k] = append(m[k], nodepool)
	}

	return func(yield func(ProviderTemplateGroup, []*spec.NodePool) bool) {
		for k, v := range m {
			if !yield(k, v) {
				return
			}
		}
	}
}

func GroupByTemplates(nps []*spec.NodePool) iter.Seq2[string, []*spec.NodePool] {
	m := make(map[string][]*spec.NodePool)

	for _, nodepool := range nps {
		np, ok := nodepool.Type.(*spec.NodePool_DynamicNodePool)
		if !ok {
			continue
		}

		p := templates.ExtractTargetPath(np.DynamicNodePool.Provider.Templates)
		m[p] = append(m[p], nodepool)
	}

	return func(yield func(string, []*spec.NodePool) bool) {
		for k, v := range m {
			if !yield(k, v) {
				return
			}
		}
	}
}
