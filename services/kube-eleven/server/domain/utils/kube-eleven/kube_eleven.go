package kube_eleven

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kube-eleven/server/domain/utils/kubeone"
	"github.com/berops/claudie/services/kube-eleven/templates"
	"github.com/rs/zerolog/log"
)

const (
	generatedKubeoneManifestName = "kubeone.yaml"
	sshKeyFileName               = "private.pem"
	baseDirectory                = "services/kube-eleven/server"
	outputDirectory              = "clusters"
	staticRegion                 = "on-premise"
	staticZone                   = "datacenter"
	staticProvider               = "on-premise"
	staticProviderName           = "claudie"
)

type KubeEleven struct {
	// Directory where files needed by Kubeone will be generated from templates.
	outputDirectory string

	// Kubernetes cluster that will be set up.
	K8sCluster *spec.K8Scluster
	// LB clusters attached to the above Kubernetes cluster.
	// If nil, the first control node becomes the api endpoint of the cluster.
	LBClusters []*spec.LBcluster

	// ProxyEnvs holds information about a need to update proxy envs, proxy endpoint, and no proxy list.
	ProxyEnvs *spec.ProxyEnvs

	// SpawnProcessLimit represents a synchronization channel which limits the number of spawned kubeone
	// processes. This values must be non-nil and be buffered, where the capacity indicates
	// the limit.
	SpawnProcessLimit chan struct{}
}

// BuildCluster is responsible for managing the given K8sCluster along with the attached LBClusters
// using Kubeone.
func (k *KubeEleven) BuildCluster() error {
	clusterID := commonUtils.GetClusterID(k.K8sCluster.ClusterInfo)

	k.outputDirectory = filepath.Join(baseDirectory, outputDirectory, clusterID)
	// Generate files which will be needed by Kubeone.
	err := k.generateFiles()
	if err != nil {
		return fmt.Errorf("error while generating files for %s : %w", k.K8sCluster.ClusterInfo.Name, err)
	}

	// Execute Kubeone apply
	kubeone := kubeone.Kubeone{
		ConfigDirectory:   k.outputDirectory,
		SpawnProcessLimit: k.SpawnProcessLimit,
	}
	err = kubeone.Apply(clusterID)
	if err != nil {
		return fmt.Errorf("error while running \"kubeone apply\" in %s : %w", k.outputDirectory, err)
	}

	// After executing Kubeone apply, the cluster kubeconfig is downloaded by kubeconfig
	// into the cluster-kubeconfig file we generated before. Now from the cluster-kubeconfig
	// we will be reading the kubeconfig of the cluster.
	kubeconfigAsString, err := readKubeconfigFromFile(filepath.Join(k.outputDirectory, fmt.Sprintf("%s-kubeconfig", k.K8sCluster.ClusterInfo.Name)))
	if err != nil {
		return fmt.Errorf("error while reading cluster-config in %s : %w", k.outputDirectory, err)
	}
	if len(kubeconfigAsString) > 0 {
		// Update kubeconfig in the target K8sCluster data structure.
		k.K8sCluster.Kubeconfig = kubeconfigAsString
	}

	// Clean up - remove generated files
	if err := os.RemoveAll(k.outputDirectory); err != nil {
		return fmt.Errorf("error while removing files from %s: %w", k.outputDirectory, err)
	}

	return nil
}

func (k *KubeEleven) DestroyCluster() error {
	clusterID := commonUtils.GetClusterID(k.K8sCluster.ClusterInfo)

	k.outputDirectory = filepath.Join(baseDirectory, outputDirectory, clusterID)

	if err := k.generateFiles(); err != nil {
		return fmt.Errorf("error while generating files for %s: %w", k.K8sCluster.ClusterInfo.Name, err)
	}

	kubeone := kubeone.Kubeone{
		ConfigDirectory:   k.outputDirectory,
		SpawnProcessLimit: k.SpawnProcessLimit,
	}

	// Destroying the cluster might fail when deleting the binaries, if its called subsequently,
	// thus ignore the error.
	if err := kubeone.Reset(clusterID); err != nil {
		log.Warn().Msgf("failed to destroy cluster and remove binaries: %s, assuming they were deleted", err)
	}

	if err := os.RemoveAll(k.outputDirectory); err != nil {
		return fmt.Errorf("error while removing files from %s: %w", k.outputDirectory, err)
	}

	return nil
}

// generateFiles will generate those files (kubeone.yaml and key.pem) needed by Kubeone.
// Returns nil if successful, error otherwise.
func (k *KubeEleven) generateFiles() error {
	// Load the Kubeone template file as *template.Template.
	template, err := templateUtils.LoadTemplate(templates.KubeOneTemplate)
	if err != nil {
		return fmt.Errorf("error while loading a kubeone template : %w", err)
	}

	// Generate templateData for the template.
	templateParameters := k.generateTemplateData()

	// Generate kubeone.yaml file from the template
	err = templateUtils.Templates{Directory: k.outputDirectory}.Generate(template, generatedKubeoneManifestName, templateParameters)
	if err != nil {
		return fmt.Errorf("error while generating %s from kubeone template : %w", generatedKubeoneManifestName, err)
	}

	if err := utils.CreateKeysForDynamicNodePools(utils.GetCommonDynamicNodePools(k.K8sCluster.ClusterInfo.NodePools), k.outputDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools: %w", err)
	}

	if err := utils.CreateKeysForStaticNodepools(utils.GetCommonStaticNodePools(k.K8sCluster.ClusterInfo.NodePools), k.outputDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	return nil
}

// generateTemplateData will create an instance of the templateData and fill up the fields
// The instance will then be returned.
func (k *KubeEleven) generateTemplateData() templateData {
	var (
		data                  templateData
		potentialEndpointNode *spec.Node
		k8sApiEndpoint        bool
	)

	data.Nodepools, potentialEndpointNode = k.getClusterNodes()
	data.APIEndpoint, k8sApiEndpoint = k.lbApiEndpointOrDefault(potentialEndpointNode)

	var alternativeNames []string
	for _, n := range k.K8sCluster.ClusterInfo.NodePools {
		if !n.IsControl {
			continue
		}
		for _, n := range n.Nodes {
			if n.NodeType != spec.NodeType_apiEndpoint {
				alternativeNames = append(alternativeNames, n.Public)
			}
		}
	}
	if k8sApiEndpoint {
		data.AlternativeNames = alternativeNames
	}

	if k.ProxyEnvs != nil && k.ProxyEnvs.UpdateProxyEnvsFlag {
		data.UtilizeHttpProxy = k.ProxyEnvs.UpdateProxyEnvsFlag
		data.NoProxyList = k.ProxyEnvs.NoProxyList
		data.HttpProxyUrl = k.ProxyEnvs.HttpProxyUrl
	}

	data.KubernetesVersion = k.K8sCluster.GetKubernetes()

	data.ClusterName = k.K8sCluster.ClusterInfo.Name

	return data
}

// getClusterNodes will parse the nodepools of k.K8sCluster and construct a slice of *NodepoolInfo.
// Returns the slice of *NodepoolInfo and the potential endpoint node.
func (k *KubeEleven) getClusterNodes() ([]*NodepoolInfo, *spec.Node) {
	nodepoolInfos := make([]*NodepoolInfo, 0, len(k.K8sCluster.ClusterInfo.NodePools))
	var endpointNode *spec.Node

	// Construct the slice of *NodepoolInfo
	for _, nodepool := range k.K8sCluster.ClusterInfo.GetNodePools() {
		var nodepoolInfo *NodepoolInfo

		if nodepool.GetDynamicNodePool() != nil {
			var nodes []*NodeInfo
			nodes, potentialEndpointNode := getNodeData(nodepool.Nodes, func(name string) string {
				return strings.TrimPrefix(name, fmt.Sprintf("%s-%s-", k.K8sCluster.ClusterInfo.Name, k.K8sCluster.ClusterInfo.Hash))
			})

			if endpointNode == nil || (potentialEndpointNode != nil && potentialEndpointNode.NodeType == spec.NodeType_apiEndpoint) {
				endpointNode = potentialEndpointNode
			}

			nodepoolInfo = &NodepoolInfo{
				NodepoolName:      nodepool.Name,
				Region:            utils.SanitiseString(nodepool.GetDynamicNodePool().Region),
				Zone:              utils.SanitiseString(nodepool.GetDynamicNodePool().Zone),
				CloudProviderName: utils.SanitiseString(nodepool.GetDynamicNodePool().Provider.CloudProviderName),
				ProviderName:      utils.SanitiseString(nodepool.GetDynamicNodePool().Provider.SpecName),
				Nodes:             nodes,
				IsDynamic:         true,
			}
		} else if nodepool.GetStaticNodePool() != nil {
			var nodes []*NodeInfo
			nodes, potentialEndpointNode := getNodeData(nodepool.Nodes, func(s string) string { return s })
			if endpointNode == nil || (potentialEndpointNode != nil && potentialEndpointNode.NodeType == spec.NodeType_apiEndpoint) {
				endpointNode = potentialEndpointNode
			}
			nodepoolInfo = &NodepoolInfo{
				NodepoolName:      nodepool.Name,
				Region:            utils.SanitiseString(staticRegion),
				Zone:              utils.SanitiseString(staticZone),
				CloudProviderName: utils.SanitiseString(staticProvider),
				ProviderName:      utils.SanitiseString(staticProviderName),
				Nodes:             nodes,
				IsDynamic:         false,
			}
		}
		nodepoolInfos = append(nodepoolInfos, nodepoolInfo)
	}

	return nodepoolInfos, endpointNode
}

// lbApiEndpointOrDefault returns the hostname of the attached api endpoint loadbalancer.
// If not present the node that is passed will be used a the default api endpoint.
// Returns the selected endpoint and a bool indicating whether the default was used or not.
func (k *KubeEleven) lbApiEndpointOrDefault(potentialEndpointNode *spec.Node) (string, bool) {
	apiEndpoint := ""

	for _, lbCluster := range k.LBClusters {
		// And if the LB cluster if of type ApiServer
		for _, role := range lbCluster.Roles {
			if role.RoleType == spec.RoleType_ApiServer {
				return lbCluster.Dns.Endpoint, false
			}
		}
	}

	// If any LB cluster of type ApiServer is not found
	// Then we will use the potential endpoint type control node.
	if potentialEndpointNode != nil {
		apiEndpoint = potentialEndpointNode.Public
		potentialEndpointNode.NodeType = spec.NodeType_apiEndpoint
	} else {
		log.Error().Msgf("Cluster %s does not have any API endpoint specified", k.K8sCluster.ClusterInfo.Name)
	}

	return apiEndpoint, true
}

// getNodeData return template data for the nodes from the cluster.
func getNodeData(nodes []*spec.Node, nameFunc func(string) string) ([]*NodeInfo, *spec.Node) {
	n := make([]*NodeInfo, 0, len(nodes))
	var potentialEndpointNode *spec.Node
	// Construct the Nodes slice inside the NodePoolInfo
	for _, node := range nodes {
		nodeName := nameFunc(node.Name)
		n = append(n, &NodeInfo{Name: nodeName, Node: node})

		// Find potential control node which can act as the cluster api endpoint
		// in case there is no LB cluster (of ApiServer type) provided in the Claudie config.

		// If cluster api endpoint is already set, use it.
		if node.GetNodeType() == spec.NodeType_apiEndpoint {
			potentialEndpointNode = node

			// otherwise choose one master node which will act as the cluster api endpoint
		} else if node.GetNodeType() == spec.NodeType_master && potentialEndpointNode == nil {
			potentialEndpointNode = node
		}
	}
	return n, potentialEndpointNode
}
