package kubeEleven

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Berops/platform/internal/templateUtils"
	"github.com/Berops/platform/internal/utils"
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/kube-eleven/server/kubeone"
	"github.com/rs/zerolog/log"
)

const (
	kubeoneTemplate = "kubeone.tpl"
	kubeoneManifest = "kubeone.yaml"
	keyFile         = "private.pem"
	kubeconfigFile  = "cluster-kubeconfig"
	baseDirectory   = "services/kube-eleven/server"
	outputDirectory = "clusters"
)

//KubeEleven struct
//K8sCluster - *pb.K8sCluster that will be set up
//LBClusters - slice of *pb.LBClusters which can be used as loadbalancer for specified K8sCluster
//			   When nil, endpoint is set to be first master node
type KubeEleven struct {
	directory  string //directory of files for kubeone
	K8sCluster *pb.K8Scluster
	LBClusters []*pb.LBcluster
}

//templateData struct is data used in template creation
type templateData struct {
	APIEndpoint string
	Kubernetes  string
	Nodes       []*pb.Node
}

//Apply will create all necessary files and apply kubeone, which will set up the cluster completely
//return nil if successful, error otherwise
func (k *KubeEleven) BuildCluster() error {
	k.directory = filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s", k.K8sCluster.ClusterInfo.Name, k.K8sCluster.ClusterInfo.Hash))
	//generate files needed for kubeone
	err := k.generateFiles()
	if err != nil {
		return fmt.Errorf("error while generating files for %s :%v", k.K8sCluster.ClusterInfo.Name, err)
	}
	//run kubeone apply
	kubeone := kubeone.Kubeone{Directory: k.directory}
	err = kubeone.Apply()
	if err != nil {
		return fmt.Errorf("error while reading cluster-config in %s: %v", k.directory, err)
	}
	// Save generated kubeconfig file to cluster config
	kc, err := readKubeconfig(filepath.Join(k.directory, kubeconfigFile))
	if err != nil {
		return fmt.Errorf("error while reading cluster-config in %s: %v", k.directory, err)

	}
	//check if kubeconfig is not empty and set it
	if len(kc) > 0 {
		k.K8sCluster.Kubeconfig = kc
		log.Info().Msgf("Kubeconfig has been saved for the cluster %s", k.K8sCluster.ClusterInfo.Name)
	}
	// Clean up
	if err := os.RemoveAll(k.directory); err != nil {
		log.Info().Msgf("error while removing files from %s: %v", k.directory, err)
		return err
	}
	log.Info().Msgf("Kube-eleven has finished setting up the cluster %s", k.K8sCluster.ClusterInfo.Name)
	return nil
}

//generateFiles will generate files needed for kubeone execution like kubeone.yaml, key.pem, etc..
//returns nil if successful, error otherwise
func (k *KubeEleven) generateFiles() error {
	template := templateUtils.Templates{Directory: k.directory}
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.KubeElevenTemplates}
	//load template file
	tpl, err := templateLoader.LoadTemplate(kubeoneTemplate)
	if err != nil {
		return fmt.Errorf("error while loading a template %s : %v", kubeoneTemplate, err)
	}
	//generate data for template
	d := k.generateTemplateData()
	//generate template
	err = template.Generate(tpl, kubeoneManifest, d)
	if err != nil {
		return fmt.Errorf("error while generating %s from %s : %v", kubeoneManifest, kubeoneTemplate, err)
	}
	// create key file
	if err := utils.CreateKeyFile(k.K8sCluster.ClusterInfo.GetPrivateKey(), k.directory, keyFile); err != nil {
		return fmt.Errorf("error while creating key file: %v", err)
	}
	// Create a cluster-kubeconfig file
	kubeconfigFilePath := filepath.Join(k.directory, kubeconfigFile)
	if err := ioutil.WriteFile(kubeconfigFilePath, []byte(k.K8sCluster.GetKubeconfig()), 0600); err != nil {
		return fmt.Errorf("error while writing cluster-kubeconfig in %s: %v", k.directory, err)
	}
	return nil
}

//generateTemplateData will create and fill up templateData with appropriate values
//return templateData with everything already set up
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
		d.APIEndpoint = d.Nodes[0].GetPublic()
	}
	return d
}

//findAPIEndpoint will loop through the slice of LBs and return endpoint, if any loadbalancer is used as API loadbalancer
//returns API endpoint if LB fulfils prerequisites, empty string otherwise
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

//getClusterNodes will parse the nodepools of the k.K8sCluster and return slice of nodes
//function also sets pb.NodeType_apiEndpoint flag if has not been set before
//returns slice of *pb.Node
func (k *KubeEleven) getClusterNodes() []*pb.Node {
	var controlNodes []*pb.Node
	var workerNodes []*pb.Node
	var ep *pb.Node
	for _, nodepool := range k.K8sCluster.ClusterInfo.GetNodePools() {
		for _, node := range nodepool.Nodes {
			if node.GetNodeType() == pb.NodeType_apiEndpoint {
				ep = node
			} else if node.GetNodeType() == pb.NodeType_master {
				controlNodes = append(controlNodes, node)
			} else {
				workerNodes = append(workerNodes, node)
			}
		}
	}
	//if no ep found, assign the first control node as API EP
	if ep == nil {
		controlNodes[0].NodeType = pb.NodeType_apiEndpoint
	}
	//in case d.Node has API endpoint node , append it to other control nodes
	controlNodes = prependNode(ep, controlNodes)
	//append all nodes and return
	return append(controlNodes, workerNodes...)
}

// readKubeconfig reads kubeconfig from a file and returns it as a string
func readKubeconfig(kubeconfigFile string) (string, error) {
	kubeconfig, err := ioutil.ReadFile(kubeconfigFile)
	if err != nil {
		return "", fmt.Errorf("error while reading kubeconfig file %s : %v", kubeconfigFile, err)
	}
	return string(kubeconfig), nil
}

//prependNode will add node to the start of the slice
//returns slice with node at the beginning
func prependNode(node *pb.Node, arr []*pb.Node) []*pb.Node {
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
