package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

const (
	outputPath      string = "services/terraformer/server/terraform"
	templatePath    string = "services/terraformer/templates"
	hostnameHashLen int    = 15
)

// flag to distinguish between different types of cluster
const (
	K8S = 0
	LB  = 1
)

// Backend struct
type Backend struct {
	ProjectName string
	ClusterName string
	DNSData     DNSData
}

// Data struct
type Data struct {
	NodePools   []*pb.NodePool
	ClusterName string
	ClusterHash string
}
type DNSData struct {
	ClusterName  string
	ClusterHash  string
	HostnameHash string
	DNSZone      string
	NodePools    []*pb.NodePool
	Project      string
	Provider     *pb.Provider
}
type jsonOut struct {
	IPs map[string]interface{} `json:"-"`
}

type DomainJSON struct {
	Domain map[string]string `json:"-"`
}

type FilePair struct {
	outputFile   string
	templateFile string
}

type ClusterPair struct {
	desiredInfo *pb.ClusterInfo
	currentInfo *pb.ClusterInfo
	clusterType int
}

func initInfra(clusterInfo *pb.ClusterInfo, backendData Backend, clusterType int) (string, error) {
	templateFilePath := filepath.Join(templatePath, "backend.tpl")
	tfFilePath := filepath.Join(outputPath, backendData.ClusterName, "backend.tf")
	outputPathCluster := filepath.Join(outputPath, backendData.ClusterName)

	// Creating backend.tf file from the template backend.tpl
	if err := templateGen(templateFilePath, tfFilePath, backendData, outputPathCluster); err != nil {
		log.Error().Msgf("Error generating terraform config file %s from template %s: %v",
			tfFilePath, templateFilePath, err)
		return "", err
	}

	// Creating .tf files for providers from templates
	if err := buildNodePools(clusterInfo, outputPathCluster, clusterType); err != nil {
		log.Error().Msgf("Error building building .tf files: %v", err)
		return "", err
	}

	// Create dns.tf files if we are dealing with loadbalancer cluster
	if clusterType == LB {
		sortedNodePools := sortNodePools(clusterInfo)
		for provider, nodepool := range sortedNodePools {
			tpl := filepath.Join(templatePath, fmt.Sprintf("%s-dns.tpl", provider))
			tf := filepath.Join(outputPath, backendData.ClusterName, fmt.Sprintf("%s-dns.tf", provider))
			backendData.DNSData.NodePools = nodepool
			err := templateGen(tpl, tf, backendData.DNSData, outputPathCluster)
			if err != nil {
				log.Error().Msgf("Error generating terraform config file %s from template %s: %v",
					tf, tpl, err)
				return "", err
			}

		}
	}

	// Create publicKey and privateKey file for a cluster
	if err := utils.CreateKeyFile(clusterInfo.GetPublicKey(), outputPathCluster, "public.pem"); err != nil {
		log.Error().Msgf("Error creating key file: %v", err)
		return "", err
	}

	// Call terraform init
	log.Info().Msgf("Running terraform init in %s", outputPathCluster)
	if err := initTerraform(outputPathCluster); err != nil {
		log.Error().Msgf("Error running terraform init in %s: %v", outputPathCluster, err)
		return "", err
	}
	return outputPathCluster, nil
}

// function will check if the hostname ends with ".", and will concatenate it if not
func getHostname(DNS *pb.DNS) string {
	if DNS.Hostname != "" {
		return DNS.Hostname
	}
	hostname := utils.CreateHash(hostnameHashLen)
	return hostname
}

func createInfra(clusterInfoDesired, clusterInfoCurrent *pb.ClusterInfo, outputPathCluster string) error {
	// terraform apply
	log.Info().Msgf("Running terraform apply in %s", outputPathCluster)
	if err := applyTerraform(outputPathCluster); err != nil {
		log.Error().Msgf("Error running terraform apply in %s: %v", outputPathCluster, err)
		return err
	}

	// group all the nodes together to make searching with respect to IP easy
	var oldNodes []*pb.Node
	if clusterInfoCurrent != nil {
		for _, oldNodepool := range clusterInfoCurrent.NodePools {
			oldNodes = append(oldNodes, oldNodepool.Nodes...)
		}
	}

	// Fill public ip addresses to NodeInfos
	for _, nodepool := range clusterInfoDesired.NodePools {
		output, err := outputTerraform(outputPathCluster, nodepool.Name)
		if err != nil {
			log.Error().Msgf("Error while getting output from terraform: %v", err)
			return err
		}

		out, err := readIPs(output)
		if err != nil {
			log.Error().Msgf("Error while reading the terraform output: %v", err)
			return err
		}
		fillNodes(&out, nodepool, oldNodes)
	}

	return nil
}

func buildClustersAsynch(desiredClusterInfo *pb.ClusterInfo, currentClusterInfo *pb.ClusterInfo, backendData Backend, clusterType int) error {
	// Prepare backend data for golang templates
	backendData.ClusterName = desiredClusterInfo.GetName() + "-" + desiredClusterInfo.GetHash()
	log.Info().Msgf("Cluster name: %s", backendData.ClusterName)

	// Create all files necessary and do terraform init
	outputPathCluster, err := initInfra(desiredClusterInfo, backendData, clusterType)
	if err != nil {
		log.Error().Msgf("Error in terraform init procedure for %s: %v",
			backendData.ClusterName, err)
		return err
	}

	// create infra via terraform plan and apply
	if err := createInfra(desiredClusterInfo, currentClusterInfo, outputPathCluster); err != nil {
		log.Error().Msgf("Error in terraform apply procedure for Loadbalancer cluster %s: %v",
			desiredClusterInfo.Name, err)
		return err
	}

	return nil
}

// buildInfrastructure is generating terraform files for different providers and calling terraform
func buildInfrastructure(currentState *pb.Project, desiredState *pb.Project) error {
	var backendData Backend
	backendData.ProjectName = desiredState.GetName()
	var errGroup errgroup.Group
	// create pairs of cluster infos
	clusterPairs := getClusterInfoPairs(desiredState.GetClusters(), currentState.GetClusters())
	clusterPairs = append(clusterPairs, getClusterInfoPairs(desiredState.GetLoadBalancerClusters(), currentState.GetLoadBalancerClusters())...)
	for _, pair := range clusterPairs {
		clusterType := pair.clusterType
		func(desiredInfo *pb.ClusterInfo, currentInfo *pb.ClusterInfo, backendData Backend) {
			if clusterType == LB {
				backendData.getDNSData(desiredState.GetLoadBalancerClusters(), pair.desiredInfo.Name)
			}
			errGroup.Go(func() error {
				err := buildClustersAsynch(desiredInfo, currentInfo, backendData, clusterType)
				if err != nil {
					log.Error().Msgf("error encountered in Terraformer - buildInfrastructure: %v", err)
					return err
				}
				return nil
			})
		}(pair.desiredInfo, pair.currentInfo, backendData)
	}
	err := errGroup.Wait()
	if err != nil {
		return fmt.Errorf("error while building infrastructure: %v", err)
	}
	// save the hostname to DNS
	for _, lbCluster := range desiredState.LoadBalancerClusters {
		for _, nodepool := range lbCluster.ClusterInfo.NodePools {
			outPath := filepath.Join(outputPath, lbCluster.ClusterInfo.Name+"-"+lbCluster.ClusterInfo.Hash)
			//use any nodepool, every single node has same domain
			outputID := fmt.Sprintf("%s-%s-%s", lbCluster.ClusterInfo.Name, lbCluster.ClusterInfo.Hash, nodepool.Name)
			output, err := outputTerraform(outPath, outputID)
			if err != nil {
				log.Error().Msgf("Error while getting output from terraform: %v", err)
				return err
			}
			out, err := readDomain(output)
			if err != nil {
				log.Error().Msgf("Error while reading the terraform output: %v", err)
				return err
			}
			domain := validateDomain(out.Domain[outputID])
			lbCluster.Dns.Hostname = domain
			log.Info().Msgf("Set the domain for %s to %s", lbCluster.ClusterInfo.Name, domain)
		}
	}
	// Clean after terraform
	if err := os.RemoveAll(outputPath + "/" + backendData.ClusterName); err != nil {
		return fmt.Errorf("error while deleting files: %v", err)
	}

	return nil
}

func validateDomain(s string) string {
	if s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}

// function returns pair of strings, first the hash hostname, second the zone
func (backend *Backend) getDNSData(lbCluster []*pb.LBcluster, lbName string) {
	for _, cluster := range lbCluster {
		if cluster.ClusterInfo.Name == lbName {
			backend.DNSData = DNSData{
				HostnameHash: getHostname(cluster.Dns),
				DNSZone:      cluster.Dns.DnsZone,
				ClusterName:  cluster.ClusterInfo.Name,
				ClusterHash:  cluster.ClusterInfo.Hash,
				Project:      cluster.Dns.Project,
				Provider:     cluster.Dns.Provider,
			}
			return
		}
	}
}

// destroyInfrastructureAsync executes terraform destroy --auto-approve. It destroys whole infrastructure in a project.
func destroyInfrastructureAsync(clusterInfo *pb.ClusterInfo, backendData Backend, clusterType int) error {
	log.Info().Msg("Generating templates for infrastructure destroy")
	backendData.ClusterName = clusterInfo.GetName() + "-" + clusterInfo.GetHash()

	log.Info().Msgf("Cluster name: %s", backendData.ClusterName)

	// Create all files necessary and do terraform init
	outputPathCluster, err := initInfra(clusterInfo, backendData, clusterType)
	if err != nil {
		log.Error().Msgf("Error in terraform init procedure for %s: %v",
			backendData.ClusterName, err)
		return err
	}

	// Call terraform destroy
	if err := destroyTerraform(outputPathCluster); err != nil {
		log.Error().Msgf("Error in destroyTerraform: %v", err)
		return err
	}

	return nil
}

func destroyInfrastructure(config *pb.Config) error {
	var backendData Backend
	backendData.ProjectName = config.GetDesiredState().GetName()
	var errGroup errgroup.Group
	// create pairs of cluster infos
	clusterPairs := getClusterInfoPairs(config.DesiredState.GetClusters(), nil)
	clusterPairs = append(clusterPairs, getClusterInfoPairs(config.DesiredState.GetLoadBalancerClusters(), nil)...)
	for _, pair := range clusterPairs {
		clusterType := pair.clusterType
		func(desiredInfo *pb.ClusterInfo, backendData Backend) {
			if clusterType == LB {
				backendData.getDNSData(config.DesiredState.LoadBalancerClusters, pair.desiredInfo.Name)
			}
			errGroup.Go(func() error {
				err := destroyInfrastructureAsync(desiredInfo, backendData, clusterType)
				if err != nil {
					log.Error().Msgf("error encountered in Terraformer - destroyInfrastructure: %v", err)
					return err
				}
				return nil
			})
		}(pair.desiredInfo, backendData)
	}
	err := errGroup.Wait()
	if err != nil {
		config.ErrorMessage = err.Error()
		return err
	}

	if err := os.RemoveAll(outputPath + "/" + backendData.ClusterName); err != nil {
		return err
	}

	return nil
}

// buildNodePools creates .tf files from providers contained in a cluster
func buildNodePools(clusterInfo *pb.ClusterInfo, outputPathCluster string, clusterType int) error {
	sortedNodePools := sortNodePools(clusterInfo)
	for providerName, nodePool := range sortedNodePools {
		log.Info().Msgf("Cluster provider: %s", providerName)
		files, err := getFilePair(clusterType)
		if err != nil {
			log.Error().Msgf("Error getting the template files: %v", err)
			return err
		}
		tplFile := filepath.Join(templatePath, fmt.Sprintf("%s%s", providerName, files.templateFile))
		tfFile := filepath.Join(outputPathCluster, fmt.Sprintf("%s%s", providerName, files.outputFile))
		err = templateGen(
			tplFile,
			tfFile,
			&Data{NodePools: nodePool, ClusterName: clusterInfo.Name, ClusterHash: clusterInfo.Hash},
			outputPathCluster)
		if err != nil {
			log.Error().Msgf("Error generating terraform config file %s from template %s: %v",
				tfFile, tplFile, err)
			return err
		}
	}
	return nil
}

// templateGen generates terraform config file from a template .tpl
func templateGen(templatePath string, tfFilePath string, d interface{}, dirName string) error {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create dir: %v", err)
		}
	}

	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to load the template file: %v", err)
	}
	log.Info().Msgf("Creating %s \n", tfFilePath)
	f, err := os.Create(tfFilePath)
	if err != nil {
		return fmt.Errorf("failed to create the %s file: %v", dirName, err)
	}

	if err := tpl.Execute(f, d); err != nil {
		return fmt.Errorf("failed to execute the template file: %v", err)
	}

	return nil
}

// initTerraform executes terraform init in a given path
func initTerraform(directoryName string) error {
	// Apply GCP credentials as an env variable
	err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "../../../../../keys/platform-296509-d6ddeb344e91.json")
	if err != nil {
		return fmt.Errorf("failed to set the google credentials env variable: %v", err)
	}
	// terraform init
	return executeTerraform(exec.Command("terraform", "init"), directoryName)
}

// applyTerraform executes terraform apply on a .tf files in a given path
func applyTerraform(directoryName string) error {
	// terraform apply --auto-approve
	return executeTerraform(exec.Command("terraform", "apply", "--auto-approve"), directoryName)
}

// destroyTerraform executes terraform destroy in a given path
func destroyTerraform(directoryName string) error {
	// terraform destroy
	return executeTerraform(exec.Command("terraform", "destroy", "--auto-approve"), directoryName)
}

func executeTerraform(cmd *exec.Cmd, workingDir string) error {
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// outputTerraform returns terraform output for a given provider and path in a json format
func outputTerraform(dirName string, name string) (string, error) {
	cmd := exec.Command("terraform", "output", "-json", name)
	cmd.Dir = dirName
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return outb.String(), nil
}

// readIPs reads json output format from terraform and unmarshal it into map[string]map[string]string readable by GO
func readIPs(data string) (jsonOut, error) {
	var result jsonOut
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result.IPs)
	return result, err
}

// readIPs reads json output format from terraform and unmarshal it into map[string]map[string]string readable by GO
func readDomain(data string) (DomainJSON, error) {
	var result DomainJSON
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result.Domain)
	return result, err
}

// getKeysFromMap returns an array of all keys in a map
func getkeysFromMap(data map[string]interface{}) []string {
	var keys []string
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// fillNodes gets ip addresses from a terraform output
func fillNodes(terraformOutput *jsonOut, newNodePool *pb.NodePool, oldNodes []*pb.Node) {
	// Fill slices from terraformOutput maps with names of nodes to ensure an order
	var tempNodes []*pb.Node

	// get sorted list of keys
	sortedNodeNames := getkeysFromMap(terraformOutput.IPs)
	for _, nodeName := range sortedNodeNames {
		var private = ""
		var control pb.NodeType

		if newNodePool.IsControl {
			control = pb.NodeType_master
		} else {
			control = pb.NodeType_worker
		}

		if len(oldNodes) > 0 {
			for _, node := range oldNodes {
				if fmt.Sprint(terraformOutput.IPs[nodeName]) == node.Public {
					private = node.Private
					control = node.NodeType
					break
				}
			}
		}
		tempNodes = append(tempNodes, &pb.Node{
			Name:     nodeName,
			Public:   fmt.Sprint(terraformOutput.IPs[nodeName]),
			Private:  private,
			NodeType: control,
		})
	}
	newNodePool.Nodes = tempNodes
}

func sortNodePools(clusterInfo *pb.ClusterInfo) map[string][]*pb.NodePool {
	sortedNodePools := map[string][]*pb.NodePool{}
	for _, nodepool := range clusterInfo.GetNodePools() {
		sortedNodePools[nodepool.Provider.Name] = append(sortedNodePools[nodepool.Provider.Name], nodepool)
	}
	return sortedNodePools
}

func getClusterInfoPairs(a, b interface{}) []ClusterPair {
	var clusterPairs []ClusterPair
	switch a.(type) {
	case []*pb.K8Scluster:
		desiredK8s := a.([]*pb.K8Scluster)
		if b == nil {
			// no current state
			for _, desired := range desiredK8s {
				clusterPairs = append(clusterPairs, ClusterPair{desired.ClusterInfo, nil, K8S})
			}
			return clusterPairs
		}
		currentK8s := b.([]*pb.K8Scluster)
		for _, desired := range desiredK8s {
			added := len(clusterPairs)
			for _, current := range currentK8s {
				if current.ClusterInfo.Name == desired.ClusterInfo.Name {
					clusterPairs = append(clusterPairs, ClusterPair{desired.ClusterInfo, current.ClusterInfo, K8S})
					break
				}
			}
			//not found in current
			if added == len(clusterPairs) {
				clusterPairs = append(clusterPairs, ClusterPair{desired.ClusterInfo, nil, K8S})
			}
		}
	case []*pb.LBcluster:
		desiredLB := a.([]*pb.LBcluster)
		if b == nil {
			// no current state
			for _, desired := range desiredLB {
				clusterPairs = append(clusterPairs, ClusterPair{desired.ClusterInfo, nil, LB})
			}
			return clusterPairs
		}
		currentLB := b.([]*pb.LBcluster)
		for _, desired := range desiredLB {
			added := len(clusterPairs)
			for _, current := range currentLB {
				if current.ClusterInfo.Name == desired.ClusterInfo.Name {
					clusterPairs = append(clusterPairs, ClusterPair{desired.ClusterInfo, current.ClusterInfo, LB})
					break
				}
			}
			//not found in current
			if added == len(clusterPairs) {
				clusterPairs = append(clusterPairs, ClusterPair{desired.ClusterInfo, nil, LB})
			}
		}
	default:
		log.Info().Msgf("Type not found in getClusterInfoPairs(): %t", a)
	}
	return clusterPairs
}

func getFilePair(clusterType int) (FilePair, error) {
	switch clusterType {
	case K8S:
		return FilePair{".tf", ".tpl"}, nil
	case LB:
		return FilePair{"-lb.tf", "-lb.tpl"}, nil
	default:
		return FilePair{}, fmt.Errorf("no such type of cluster")
	}
}
