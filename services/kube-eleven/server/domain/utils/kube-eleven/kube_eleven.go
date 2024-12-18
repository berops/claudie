package kube_eleven

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/internal/sanitise"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kube-eleven/server/domain/utils/kubeone"
	"github.com/berops/claudie/services/kube-eleven/templates"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
)

const (
	generatedKubeoneManifestName = "kubeone.yaml"
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

	// SpawnProcessLimit limits the number of spawned kubeone processes.
	SpawnProcessLimit *semaphore.Weighted
}

// BuildCluster is responsible for managing the given K8sCluster along with the attached LBClusters
// using Kubeone.
func (k *KubeEleven) BuildCluster() error {
	clusterID := k.K8sCluster.ClusterInfo.Id()

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
	clusterID := k.K8sCluster.ClusterInfo.Id()

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

	if err := nodepools.DynamicGenerateKeys(nodepools.Dynamic(k.K8sCluster.ClusterInfo.NodePools), k.outputDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools: %w", err)
	}

	if err := nodepools.StaticGenerateKeys(nodepools.Static(k.K8sCluster.ClusterInfo.NodePools), k.outputDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	return nil
}

// generateTemplateData will create an instance of the templateData and fill up the fields
// The instance will then be returned.
func (k *KubeEleven) generateTemplateData() templateData {
	var (
		data           templateData
		k8sApiEndpoint bool
	)

	data.Nodepools = k.getClusterNodes()
	data.APIEndpoint, k8sApiEndpoint = k.apiEndpoint()

	var alternativeNames []string
	for n := range nodepools.Control(k.K8sCluster.ClusterInfo.NodePools) {
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
func (k *KubeEleven) getClusterNodes() []*NodepoolInfo {
	nodepoolInfos := make([]*NodepoolInfo, 0, len(k.K8sCluster.ClusterInfo.NodePools))

	for _, nodepool := range k.K8sCluster.ClusterInfo.GetNodePools() {
		var nodepoolInfo *NodepoolInfo

		if nodepool.GetDynamicNodePool() != nil {
			nodepoolInfo = &NodepoolInfo{
				NodepoolName:      nodepool.Name,
				Region:            sanitise.String(nodepool.GetDynamicNodePool().Region),
				Zone:              sanitise.String(nodepool.GetDynamicNodePool().Zone),
				CloudProviderName: sanitise.String(nodepool.GetDynamicNodePool().Provider.CloudProviderName),
				ProviderName:      sanitise.String(nodepool.GetDynamicNodePool().Provider.SpecName),
				Nodes: getNodeData(nodepool.Nodes, func(name string) string {
					return strings.TrimPrefix(name, fmt.Sprintf("%s-", k.K8sCluster.ClusterInfo.Id()))
				}),
				IsDynamic: true,
			}
		} else if nodepool.GetStaticNodePool() != nil {
			nodepoolInfo = &NodepoolInfo{
				NodepoolName:      nodepool.Name,
				Region:            sanitise.String(staticRegion),
				Zone:              sanitise.String(staticZone),
				CloudProviderName: sanitise.String(staticProvider),
				ProviderName:      sanitise.String(staticProviderName),
				Nodes:             getNodeData(nodepool.Nodes, func(s string) string { return s }),
				IsDynamic:         false,
			}
		}
		nodepoolInfos = append(nodepoolInfos, nodepoolInfo)
	}

	return nodepoolInfos
}

// apiEndpoint will extract the publicly accessible endpoint for the api server, which can
// be either a loadbalancer or a control plane node directly.
func (k *KubeEleven) apiEndpoint() (string, bool) {
	for _, lbCluster := range k.LBClusters {
		if lbCluster.HasApiRole() {
			return lbCluster.Dns.Endpoint, false
		}
	}
	_, n := nodepools.FindApiEndpoint(k.K8sCluster.ClusterInfo.NodePools)
	if n == nil {
		// This should never happen as the apiEndpoint role is always chosen by the manager service.
		panic("malformed k8s state, no loadbalancer attach with api role nor any control plane node has api server role")
	}
	return n.Public, true
}

// getNodeData return template data for the nodes from the cluster.
func getNodeData(nodes []*spec.Node, nameFunc func(string) string) []*NodeInfo {
	n := make([]*NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		nodeName := nameFunc(node.Name)
		n = append(n, &NodeInfo{Name: nodeName, Node: node})
	}
	return n
}
