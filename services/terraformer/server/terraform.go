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
	outputPath   string = "services/terraformer/server/terraform"
	templatePath string = "services/terraformer/templates"
)

// Backend struct
type Backend struct {
	ProjectName string
	ClusterName string
}

// Data struct
type Data struct {
	NodePools   []*pb.NodePool
	ClusterName string
	ClusterHash string
}
type jsonOut struct {
	IPs map[string]interface{} `json:"-"`
}

func initInfra(clusterInfo *pb.ClusterInfo, backendData Backend, tplFile, tfFile string) (string, error) {
	// Creating backend.tf file
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
	if err := buildNodePools(clusterInfo, outputPathCluster, tplFile, tfFile); err != nil {
		return "", err
	}

	// Create publicKey and privateKey file for a cluster
	if err := utils.CreateKeyFile(clusterInfo.PublicKey, outputPathCluster, "public.pem"); err != nil {
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
		output, err := outputTerraform(outputPathCluster, nodepool)
		if err != nil {
			return err
		}

		out, err := readOutput(output)
		if err != nil {
			return err
		}
		fmt.Printf("%v", out)
		fillNodes(&out, nodepool, oldNodes)
	}
	return nil
}

func buildK8sClustersAsynch(desiredStateCluster *pb.K8Scluster, currentStateCluster *pb.K8Scluster, backendData Backend) error {
	// Prepare backend data for golang templates
	backendData.ClusterName = desiredStateCluster.ClusterInfo.GetName() + "-" + desiredStateCluster.ClusterInfo.GetHash()
	log.Info().Msgf("Cluster name: %s", backendData.ClusterName)

	// Create all files necessary and do terraform init
	outputPathCluster, err := initInfra(desiredStateCluster.ClusterInfo, backendData, ".tpl", ".tf")
	if err != nil {
		log.Error().Msgf("Error in terraform init procedure for %s: %v",
			backendData.ClusterName, err)
		return err
	}
	var currentClusterInfo *pb.ClusterInfo
	if currentStateCluster != nil {
		currentClusterInfo = currentStateCluster.ClusterInfo
	}

	if err := createInfra(desiredStateCluster.ClusterInfo, currentClusterInfo, outputPathCluster); err != nil {
		log.Error().Msgf("Error in terraform apply procedure for Loadbalancer cluster %s: %v",
			desiredStateCluster.ClusterInfo.Name, err)
		return err
	}

	return nil
}

func buildLBClustersAsynch(desiredLbCluster *pb.LBcluster, currentLbCluster *pb.LBcluster, backendData Backend) error {
	backendData.ClusterName = desiredLbCluster.ClusterInfo.GetName() + "-" + desiredLbCluster.ClusterInfo.GetHash()
	// Create .tf files
	outputPathCluster, err := initInfra(desiredLbCluster.ClusterInfo, backendData, "-lb.tpl", "-lb.tf")
	if err != nil {
		log.Error().Msgf("Error in terraform init procedure for Loadbalancer cluster %s: %v",
			desiredLbCluster.ClusterInfo.Name, err)
		return err
	}
	var currentClusterInfo *pb.ClusterInfo
	//check if any current cluster exists
	if currentLbCluster != nil {
		currentClusterInfo = currentLbCluster.ClusterInfo
	}
	if err := createInfra(desiredLbCluster.ClusterInfo, currentClusterInfo, outputPathCluster); err != nil {
		log.Error().Msgf("Error in terraform apply procedure for Loadbalancer cluster %s: %v",
			desiredLbCluster.ClusterInfo.Name, err)
		return err
	}
	return nil
}

// buildInfrastructure is generating terraform files for different providers and calling terraform
func buildInfrastructure(currentState *pb.Project, desiredState *pb.Project) error {
	fmt.Println("Generating templates")
	var backendData Backend
	backendData.ProjectName = desiredState.GetName()
	var errGroup errgroup.Group

	for _, desiredStateCluster := range desiredState.GetClusters() {
		var oldCluster *pb.K8Scluster
		for _, currentStateCluster := range currentState.GetClusters() {
			if currentStateCluster.ClusterInfo.Name == desiredStateCluster.ClusterInfo.Name {
				oldCluster = currentStateCluster
				break
			}
		}
		func(desiredStateCluster *pb.K8Scluster, currentStateCluster *pb.K8Scluster, backendData Backend) {
			errGroup.Go(func() error {
				err := buildK8sClustersAsynch(desiredStateCluster, currentStateCluster, backendData)
				if err != nil {
					log.Error().Msgf("error encountered in Terraformer - buildInfrastructure: %v", err)
					return err
				}
				return nil
			})
		}(desiredStateCluster, oldCluster, backendData)
	}

	for _, desiredLbCluster := range desiredState.GetLoadBalancerClusters() {
		var oldCluster *pb.LBcluster
		for _, currentLbCluster := range currentState.GetLoadBalancerClusters() {
			if currentLbCluster.ClusterInfo.Name == desiredLbCluster.ClusterInfo.Name {
				oldCluster = currentLbCluster
				break
			}
		}
		func(desiredLbCluster *pb.LBcluster, currentLbCluster *pb.LBcluster, backendData Backend) {
			errGroup.Go(func() error {
				err := buildLBClustersAsynch(desiredLbCluster, currentLbCluster, backendData)
				if err != nil {
					log.Error().Msgf("error encountered in Terraformer - buildInfrastructure: %v", err)
					return err
				}
				return nil
			})
		}(desiredLbCluster, oldCluster, backendData)
	}
	err := errGroup.Wait()
	if err != nil {
		return err
	}

	// Clean after terraform
	if err := os.RemoveAll(outputPath + "/" + backendData.ClusterName); err != nil {
		return err
	}

	return nil
}

// destroyInfrastructureAsync executes terraform destroy --auto-approve. It destroys whole infrastructure in a project.
func destroyInfrastructureAsync(clusterInfo *pb.ClusterInfo, backendData Backend, tfFile, tplFile string) error {
	log.Info().Msg("Generating templates for infrastructure destroy")
	backendData.ClusterName = clusterInfo.GetName() + "-" + clusterInfo.GetHash()

	log.Info().Msgf("Cluster name: %s", backendData.ClusterName)

	// Create all files necessary and do terraform init
	outputPathCluster, err := initInfra(clusterInfo, backendData, tplFile, tfFile)
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
	fmt.Println("Generating templates")
	var backendData Backend
	backendData.ProjectName = config.GetDesiredState().GetName()
	var errGroup errgroup.Group

	// Destroy K8s clusters
	for _, cluster := range config.GetDesiredState().GetClusters() {
		func(clusterInfo *pb.ClusterInfo, backendData Backend) {
			errGroup.Go(func() error {
				err := destroyInfrastructureAsync(clusterInfo, backendData, ".tf", ".tpl")
				if err != nil {
					log.Error().Msgf("error encountered in Terraformer - destroyInfrastructure: %v", err)
					config.ErrorMessage = err.Error()
					return err
				}
				return nil
			})
		}(cluster.ClusterInfo, backendData)
	}
	// Destroy LB clusters
	for _, cluster := range config.GetDesiredState().GetLoadBalancerClusters() {
		func(clusterInfo *pb.ClusterInfo, backendData Backend) {
			errGroup.Go(func() error {
				err := destroyInfrastructureAsync(clusterInfo, backendData, "-lb.tf", "-lb.tpl")
				if err != nil {
					log.Error().Msgf("error encountered in Terraformer - destroyInfrastructure: %v", err)
					config.ErrorMessage = err.Error()
					return err
				}
				return nil
			})
		}(cluster.ClusterInfo, backendData)
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
func buildNodePools(clusterInfo *pb.ClusterInfo, outputPathCluster string, tplFile, tfFile string) error {
	sortedNodePools := sortNodePools(clusterInfo)
	for providerName, nodePool := range sortedNodePools {
		log.Info().Msgf("Cluster provider: %s", providerName)
		tplFileName := fmt.Sprintf("%s%s", providerName, tplFile)
		terraformFileName := fmt.Sprintf("%s%s", providerName, tfFile)
		tplFile := filepath.Join(templatePath, tplFileName)
		trfFile := filepath.Join(outputPathCluster, terraformFileName)
		err := templateGen(
			tplFile,
			trfFile,
			&Data{NodePools: nodePool, ClusterName: clusterInfo.Name, ClusterHash: clusterInfo.Hash},
			outputPathCluster)
		if err != nil {
			log.Error().Msgf("Error generating terraform config file %s from template %s: %v",
				trfFile, tplFile, err)
			return err
		}
	}
	return nil
}

// templateGen generates terraform config file from a template .tpl
func templateGen(templatePath string, outputPath string, d interface{}, dirName string) error {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create dir: %v", err)
		}
	}

	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to load the template file: %v", err)
	}
	fmt.Printf("Creating %s \n", outputPath)
	f, err := os.Create(outputPath)
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
func outputTerraform(dirName string, nodepool *pb.NodePool) (string, error) {
	cmd := exec.Command("terraform", "output", "-json", nodepool.Name)
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

// readOutput reads json output format from terraform and unmarshal it into map[string]map[string]string readable by GO
func readOutput(data string) (jsonOut, error) {
	var result jsonOut
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result.IPs)
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
			control = 1
		} else {
			control = 0
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
