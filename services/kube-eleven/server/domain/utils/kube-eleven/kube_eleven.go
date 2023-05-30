package kube_eleven

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kube-eleven/server/domain/utils/kubeone"
	"github.com/berops/claudie/services/kube-eleven/templates"
)

const (
	generatedKubeoneManifestName = "kubeone.yaml"
	sshKeyFileName               = "private.pem"
	kubeconfigFileName           = "cluster-kubeconfig"
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
	K8sCluster *pb.K8Scluster
	// LB clusters attached to the above Kubernetes cluster.
	// If nil, the first control node becomes the api endpoint of the cluster.
	LBClusters []*pb.LBcluster
}

// BuildCluster is responsible for managing the given K8sCluster along with the attached LBClusters
// using Kubeone.
func (k *KubeEleven) BuildCluster() error {
	clusterID := fmt.Sprintf("%s-%s", k.K8sCluster.ClusterInfo.Name, k.K8sCluster.ClusterInfo.Hash)

	k.outputDirectory = filepath.Join(baseDirectory, outputDirectory, clusterID)
	// Generate files which will be needed by Kubeone.
	err := k.generateFiles()
	if err != nil {
		return fmt.Errorf("error while generating files for %s : %w", k.K8sCluster.ClusterInfo.Name, err)
	}

	// Execute Kubeone apply
	kubeone := kubeone.Kubeone{ConfigDirectory: k.outputDirectory}
	err = kubeone.Apply(clusterID)
	if err != nil {
		return fmt.Errorf("error while running \"kubeone apply\" in %s : %w", k.outputDirectory, err)
	}

	// After executing Kubeone apply, the cluster kubeconfig is downloaded by kubeconfig
	// into the cluster-kubeconfig file we generated before. Now from the cluster-kubeconfig
	// we will be reading the kubeconfig of the cluster.
	kubeconfigAsString, err := readKubeconfigFromFile(filepath.Join(k.outputDirectory, kubeconfigFileName))
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

	// Create file containing SSH key which will be used by Kubeone.
	if err := utils.CreateKeyFile(k.K8sCluster.ClusterInfo.GetPrivateKey(), k.outputDirectory, sshKeyFileName); err != nil {
		return fmt.Errorf("error while creating SSH key file: %w", err)
	}

	// Create a kubeconfig file for the target Kubernetes cluster.
	kubeconfigFilePath := filepath.Join(k.outputDirectory, kubeconfigFileName)
	if err := os.WriteFile(kubeconfigFilePath, []byte(k.K8sCluster.GetKubeconfig()), 0600); err != nil {
		return fmt.Errorf("error while writing cluster-kubeconfig file in %s: %w", k.outputDirectory, err)
	}

	return nil
}

// generateTemplateData will create an instance of the templateData and fill up the fields
// The instance will then be returned.
func (k *KubeEleven) generateTemplateData() templateData {
	var data templateData

	var potentialEndpointNode *pb.Node
	data.Nodepools, potentialEndpointNode = k.getClusterNodes()

	data.APIEndpoint = k.findAPIEndpoint(potentialEndpointNode)

	data.KubernetesVersion = k.K8sCluster.GetKubernetes()

	return data
}

// getClusterNodes will parse the nodepools of k.K8sCluster and construct a slice of *NodepoolInfo.
// Returns the slice of *NodepoolInfo and the potential endpoint node.
func (k *KubeEleven) getClusterNodes() ([]*NodepoolInfo, *pb.Node) {
	nodepoolInfos := make([]*NodepoolInfo, 0, len(k.K8sCluster.ClusterInfo.NodePools))
	var potentialEndpointNode *pb.Node

	// Construct the slice of *Nodepoolnfo
	for _, nodepool := range k.K8sCluster.ClusterInfo.GetNodePools() {
		var nodepoolInfo *NodepoolInfo

		if nodepool.GetDynamicNodePool() != nil {
			var nodes []*NodeInfo
			nodes, potentialEndpointNode = getNodeData(nodepool.GetDynamicNodePool().Nodes, func(name string) string {
				return strings.TrimPrefix(name, fmt.Sprintf("%s-%s-", k.K8sCluster.ClusterInfo.Name, k.K8sCluster.ClusterInfo.Hash))
			})
			nodepoolInfo = &NodepoolInfo{
				NodepoolName:      nodepool.GetDynamicNodePool().Name,
				Region:            sanitiseString(nodepool.GetDynamicNodePool().Region),
				Zone:              sanitiseString(nodepool.GetDynamicNodePool().Zone),
				CloudProviderName: sanitiseString(nodepool.GetDynamicNodePool().Provider.CloudProviderName),
				ProviderName:      sanitiseString(nodepool.GetDynamicNodePool().Provider.SpecName),
				Nodes:             nodes,
			}

		} else if nodepool.GetStaticNodePool() != nil {
			var nodes []*NodeInfo
			nodes, potentialEndpointNode = getNodeData(nodepool.GetStaticNodePool().Nodes, func(s string) string { return s })
			nodepoolInfo = &NodepoolInfo{
				NodepoolName:      nodepool.GetStaticNodePool().Name,
				Region:            sanitiseString(staticRegion),
				Zone:              sanitiseString(staticZone),
				CloudProviderName: sanitiseString(staticProvider),
				ProviderName:      sanitiseString(staticProviderName),
				Nodes:             nodes,
			}
		}
		nodepoolInfos = append(nodepoolInfos, nodepoolInfo)
	}

	return nodepoolInfos, potentialEndpointNode
}

// findAPIEndpoint returns the cluster api endpoint.
// It loops through the slice of attached LB clusters and if any ApiServer type LB cluster is found,
// then it's DNS endpoint is returned as the cluster api endpoint.
// Otherwise returns the public IP of the potential endpoint node found in getClusterNodes( ).
func (k *KubeEleven) findAPIEndpoint(potentialEndpointNode *pb.Node) string {
	apiEndpoint := ""

	for _, lbCluster := range k.LBClusters {
		// If the LB cluster is attached to out target Kubernetes cluster
		if lbCluster.TargetedK8S == k.K8sCluster.ClusterInfo.Name {
			// And if the LB cluster if of type ApiServer
			for _, role := range lbCluster.Roles {
				if role.RoleType == pb.RoleType_ApiServer {
					return lbCluster.Dns.Endpoint
				}
			}
		}
	}

	// If any LB cluster of type ApiServer is not found
	// Then we will use the potential endpoint type control node.
	if potentialEndpointNode != nil {
		apiEndpoint = potentialEndpointNode.Public
		potentialEndpointNode.NodeType = pb.NodeType_apiEndpoint
	} else {
		log.Error().Msgf("Cluster %s does not have any API endpoint specified", k.K8sCluster.ClusterInfo.Name)
	}

	return apiEndpoint
}

func getNodeData(nodes []*pb.Node, nameFunc func(string) string) ([]*NodeInfo, *pb.Node) {
	n := make([]*NodeInfo, 0, len(nodes))
	var potentialEndpointNode *pb.Node
	// Construct the Nodes slice inside the NodePoolInfo
	for _, node := range nodes {
		nodeName := nameFunc(node.Name)
		n = append(n, &NodeInfo{Name: nodeName, Node: node})

		// Find potential control node which can act as the cluster api endpoint
		// in case there is no LB cluster (of ApiServer type) provided in the Claudie config.

		// If cluster api endpoint is already set, use it.
		if node.GetNodeType() == pb.NodeType_apiEndpoint {
			potentialEndpointNode = node

			// otherwise choose one master node which will act as the cluster api endpoint
		} else if node.GetNodeType() == pb.NodeType_master && potentialEndpointNode == nil {
			potentialEndpointNode = node
		}
	}
	return n, potentialEndpointNode
}
