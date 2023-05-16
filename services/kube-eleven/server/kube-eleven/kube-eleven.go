package kubeEleven

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kube-eleven/server/kubeone"
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

type NodepoolInfo struct {
	Nodes             []*NodeInfo
	NodepoolName      string
	Region            string
	Zone              string
	CloudProviderName string
	ProviderName      string
}

// templateData struct is data used in template creation
type templateData struct {
	APIEndpoint string
	Kubernetes  string
	Nodepools   []*NodepoolInfo
}

// Apply will create all necessary files and apply kubeone, which will set up the cluster completely
// return nil if successful, error otherwise
func (k *KubeEleven) BuildCluster() error {
	clusterID := fmt.Sprintf("%s-%s", k.K8sCluster.ClusterInfo.Name, k.K8sCluster.ClusterInfo.Hash)
	k.directory = filepath.Join(baseDirectory, outputDirectory, clusterID)
	//generate files needed for kubeone
	err := k.generateFiles()
	if err != nil {
		return fmt.Errorf("error while generating files for %s : %w", k.K8sCluster.ClusterInfo.Name, err)
	}
	//run kubeone apply
	kubeone := kubeone.Kubeone{Directory: k.directory}
	err = kubeone.Apply(clusterID)
	if err != nil {
		return fmt.Errorf("error while running \"kubeone apply\" in %s : %w", k.directory, err)
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
		return fmt.Errorf("error while removing files from %s: %w", k.directory, err)
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
	var ep *pb.Node
	//Prepare the nodes for template
	d.Nodepools, ep = k.getClusterNodes()
	// Get the API endpoint. If it is not set, use the first control node
	d.APIEndpoint = k.findAPIEndpoint(ep)
	//save k8s version
	d.Kubernetes = k.K8sCluster.GetKubernetes()
	return d
}

// findAPIEndpoint loops through the slice of LBs and return endpoint, if any loadbalancer is used as API loadbalancer.
// Returns API endpoint if LB fulfils prerequisites, if not, returns the public IP of the node provided.
func (k *KubeEleven) findAPIEndpoint(ep *pb.Node) string {
	apiEndpoint := ""
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

	if ep != nil {
		apiEndpoint = ep.Public
		ep.NodeType = pb.NodeType_apiEndpoint
	} else {
		log.Err(fmt.Errorf("cluster does not have any API endpoint specified")).Str("cluster", k.K8sCluster.ClusterInfo.Name).Msgf("Error while looking for API endpoint")
	}

	return apiEndpoint
}

// getClusterNodes will parse the nodepools of the k.K8sCluster and return slice of node, and potential endpoint node
// returns slice of *pb.Node
func (k *KubeEleven) getClusterNodes() ([]*NodepoolInfo, *pb.Node) {
	nodepoolInfos := make([]*NodepoolInfo, 0, len(k.K8sCluster.ClusterInfo.NodePools))
	var ep *pb.Node
	// build template data
	for _, nodepool := range k.K8sCluster.ClusterInfo.GetNodePools() {
		nodepoolInfo := &NodepoolInfo{
			NodepoolName:      nodepool.Name,
			Region:            sanitiseLabel(nodepool.Region),
			Zone:              sanitiseLabel(nodepool.Zone),
			CloudProviderName: sanitiseLabel(nodepool.Provider.CloudProviderName),
			ProviderName:      sanitiseLabel(nodepool.Provider.SpecName),
			Nodes:             make([]*NodeInfo, 0, len(nodepool.Nodes)),
		}
		for _, node := range nodepool.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-%s-", k.K8sCluster.ClusterInfo.Name, k.K8sCluster.ClusterInfo.Hash))
			nodepoolInfo.Nodes = append(nodepoolInfo.Nodes, &NodeInfo{Name: nodeName, Node: node})
			// save API endpoint in case there is no LB
			if node.GetNodeType() == pb.NodeType_apiEndpoint {
				// if endpoint set, use it
				ep = node
			} else if node.GetNodeType() == pb.NodeType_master && ep == nil {
				//if no endpoint set, choose one master node
				ep = node
			}
		}
		nodepoolInfos = append(nodepoolInfos, nodepoolInfo)
	}

	return nodepoolInfos, ep
}

// readKubeconfig reads kubeconfig from a file and returns it as a string
func readKubeconfig(kubeconfigFile string) (string, error) {
	kubeconfig, err := os.ReadFile(kubeconfigFile)
	if err != nil {
		return "", fmt.Errorf("error while reading kubeconfig file %s : %w", kubeconfigFile, err)
	}
	return string(kubeconfig), nil
}

// sanitiseLabel replaces all white spaces and ":" in the string to "-".
func sanitiseLabel(s string) string {
	// convert to lower case
	sanitised := strings.ToLower(s)
	// replace all white space with "-"
	sanitised = strings.ReplaceAll(sanitised, " ", "-")
	// replace all ":" with "-"
	sanitised = strings.ReplaceAll(sanitised, ":", "-")
	return sanitised
}
