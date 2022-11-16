package kubeEleven

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Berops/claudie/internal/templateUtils"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	"github.com/Berops/claudie/services/kube-eleven/server/kubeone"
)

const (
	kubeoneTemplate = "kubeone.tpl"
	kubeoneManifest = "kubeone.yaml"
	keyFile         = "private.pem"
	kubeconfigFile  = "cluster-kubeconfig"
	baseDirectory   = "services/kube-eleven/server"
	outputDirectory = "clusters"
)

// KubeEleven struct
// K8sCluster - *pb.K8sCluster that will be set up
// LBClusters - slice of *pb.LBClusters which can be used as loadbalancer for specified K8sCluster
//
//	When nil, endpoint is set to be first master node
type KubeEleven struct {
	directory  string //directory of files for kubeone
	K8sCluster *pb.K8Scluster
	LBClusters []*pb.LBcluster
}

type NodeInfo struct {
	Node *pb.Node
	Name string
}

// templateData struct is data used in template creation
type templateData struct {
	APIEndpoint string
	Kubernetes  string
	Nodes       []*NodeInfo
}

// Apply will create all necessary files and apply kubeone, which will set up the cluster completely
// return nil if successful, error otherwise
func (k *KubeEleven) BuildCluster() error {
	k.directory = filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", k.K8sCluster.ClusterInfo.Name, k.K8sCluster.ClusterInfo.Hash))
	//generate files needed for kubeone
	err := k.generateFiles()
	if err != nil {
		return fmt.Errorf("error while generating files for %s : %w", k.K8sCluster.ClusterInfo.Name, err)
	}
	//run kubeone apply
	kubeone := kubeone.Kubeone{Directory: k.directory}
	err = kubeone.Apply()
	if err != nil {
		return fmt.Errorf("error while reading cluster-config in %s : %w", k.directory, err)
	}
	// Save generated kubeconfig file to cluster config
	kc, err := readKubeconfig(filepath.Join(k.directory, kubeconfigFile))
	if err != nil {
		return fmt.Errorf("error while reading cluster-config in %s : %w", k.directory, err)

	}
	//check if kubeconfig is not empty and set it
	if len(kc) > 0 {
		k.K8sCluster.Kubeconfig = kc
	}
	// Clean up
	if err := os.RemoveAll(k.directory); err != nil {
		return fmt.Errorf("error while removing files from %s: %v", k.directory, err)
	}
	return nil
}

// generateFiles will generate files needed for kubeone execution like kubeone.yaml, key.pem, etc..
// returns nil if successful, error otherwise
func (k *KubeEleven) generateFiles() error {
	template := templateUtils.Templates{Directory: k.directory}
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.KubeElevenTemplates}
	//load template file
	tpl, err := templateLoader.LoadTemplate(kubeoneTemplate)
	if err != nil {
		return fmt.Errorf("error while loading a template %s : %w", kubeoneTemplate, err)
	}
	//generate data for template
	d := k.generateTemplateData()
	//generate template
	err = template.Generate(tpl, kubeoneManifest, d)
	if err != nil {
		return fmt.Errorf("error while generating %s from %s : %w", kubeoneManifest, kubeoneTemplate, err)
	}
	// create key file
	if err := utils.CreateKeyFile(k.K8sCluster.ClusterInfo.GetPrivateKey(), k.directory, keyFile); err != nil {
		return fmt.Errorf("error while creating key file: %w", err)
	}
	// Create a cluster-kubeconfig file
	kubeconfigFilePath := filepath.Join(k.directory, kubeconfigFile)
	if err := os.WriteFile(kubeconfigFilePath, []byte(k.K8sCluster.GetKubeconfig()), 0600); err != nil {
		return fmt.Errorf("error while writing cluster-kubeconfig in %s: %w", k.directory, err)
	}
	return nil
}

// generateTemplateData will create and fill up templateData with appropriate values
// return templateData with everything already set up
func (k *KubeEleven) generateTemplateData() templateData {
	var d templateData
	// Get the API endpoint. If it is not set, use the first control node
	d.APIEndpoint = k.findAPIEndpoint()
	//Prepare the nodes for template
	d.Nodes = k.getClusterNodes()

	//save k8s version
	d.Kubernetes = k.K8sCluster.GetKubernetes()
	//set up API EP if not set by lb
	if d.APIEndpoint == "" {
		d.APIEndpoint = d.Nodes[0].Node.GetPublic()
	}
	return d
}

// findAPIEndpoint will loop through the slice of LBs and return endpoint, if any loadbalancer is used as API loadbalancer
// returns API endpoint if LB fulfils prerequisites, empty string otherwise
func (k *KubeEleven) findAPIEndpoint() string {
	for _, lbCluster := range k.LBClusters {
		//check if lb is used for this k8s
		if lbCluster.TargetedK8S == k.K8sCluster.ClusterInfo.Name {
			//check if the lb is api-lb
			for _, role := range lbCluster.Roles {
				if role.RoleType == pb.RoleType_ApiServer {
					return lbCluster.Dns.Endpoint
				}
			}
		}
	}
	return ""
}

// getClusterNodes will parse the nodepools of the k.K8sCluster and return slice of nodes
// function also sets pb.NodeType_apiEndpoint flag if has not been set before
// returns slice of *pb.Node
func (k *KubeEleven) getClusterNodes() []*NodeInfo {
	var controlNodes []*NodeInfo
	var workerNodes []*NodeInfo
	var ep *NodeInfo
	for _, nodepool := range k.K8sCluster.ClusterInfo.GetNodePools() {
		for i, node := range nodepool.Nodes {
			nodeName := fmt.Sprintf("%s-%d", nodepool.Name, i+1)
			if node.GetNodeType() == pb.NodeType_apiEndpoint {
				ep = &NodeInfo{Name: nodeName, Node: node}
			} else if node.GetNodeType() == pb.NodeType_master {
				controlNodes = append(controlNodes, &NodeInfo{Name: nodeName, Node: node})
			} else {
				workerNodes = append(workerNodes, &NodeInfo{Name: nodeName, Node: node})
			}
		}
	}
	//if no ep found, assign the first control node as API EP
	if ep == nil {
		controlNodes[0].Node.NodeType = pb.NodeType_apiEndpoint
	}
	//in case d.Node has API endpoint node , append it to other control nodes
	controlNodes = prependNode(ep, controlNodes)
	//append all nodes and return
	return append(controlNodes, workerNodes...)
}

// readKubeconfig reads kubeconfig from a file and returns it as a string
func readKubeconfig(kubeconfigFile string) (string, error) {
	kubeconfig, err := os.ReadFile(kubeconfigFile)
	if err != nil {
		return "", fmt.Errorf("error while reading kubeconfig file %s : %w", kubeconfigFile, err)
	}
	return string(kubeconfig), nil
}

// prependNode will add node to the start of the slice
// returns slice with node at the beginning
func prependNode(node *NodeInfo, arr []*NodeInfo) []*NodeInfo {
	if node == nil {
		return arr
	}
	//put node at the end
	arr = append(arr, node)
	//shift elements towards the end
	copy(arr[1:], arr)
	//set element as first
	arr[0] = node
	return arr
}
