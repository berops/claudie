package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"text/template"

	"github.com/Berops/platform/proto/pb"
)

const outputPath string = "services/terraformer/server/terraform"
const templatePath string = "services/terraformer/templates"

type Backend struct {
	ProjectName string
	ClusterName string
}

type Data struct {
	Index   int
	Cluster *pb.Cluster
}

func createKeyFile(key string, keyType string) error {
	return ioutil.WriteFile(outputPath+keyType, []byte(key), 0600)
}

// buildInfrastructure is generating terraform files for different providers and calling terraform
func buildInfrastructure(config *pb.Config) error {
	desiredState := config.DesiredState
	fmt.Println("Generating templates")
	var backendData Backend
	backendData.ProjectName = desiredState.GetName()
	for _, cluster := range desiredState.Clusters {
		log.Println("Cluster name:", cluster.GetName())
		backendData.ClusterName = cluster.GetName()
		// Creating backend.tf file from the template
		if err := templateGen(templatePath+"/backend.tpl", outputPath+"/backend.tf", backendData, outputPath); err != nil {
			return err
		}
		// Creating .tf files for providers from templates
		if err := buildNodePools(cluster); err != nil {
			return err
		}
		// Create publicKey file for a cluster
		if err := createKeyFile(cluster.GetPublicKey(), "/public.pem"); err != nil {
			return err
		}

		if err := createKeyFile(cluster.GetPublicKey(), "/private.pem"); err != nil {
			return err
		}
		// Call terraform init and apply
		if err := initTerraform(outputPath); err != nil {
			return err
		}

		if err := applyTerraform(outputPath); err != nil {
			return err
		}

		// Fill public ip addresses
		var m map[string]*pb.Ip
		tmpCluster := getClusterByName(cluster.Name, config.CurrentState.Clusters)

		if tmpCluster == nil {
			m = make(map[string]*pb.Ip)
		} else {
			m = tmpCluster.Ips
		}
		for _, nodepool := range cluster.NodePools {
			output, err := outputTerraform(outputPath, nodepool.Provider.Name)
			if err != nil {
				return err
			}

			out, err := readOutput(output)
			if err != nil {
				return err
			}
			m = fillNodes(m, out, nodepool)
		}
		cluster.Ips = m
		// Clean after Terraform. Remove tmp terraform dir.
		err := os.RemoveAll("services/terraformer/server/terraform")
		if err != nil {
			return err
		}
	}

	for _, m := range desiredState.Clusters {
		log.Println(m.Ips)
	}

	return nil
}

func getClusterByName(name string, clusters []*pb.Cluster) *pb.Cluster {
	if name == "" {
		return nil
	}
	if len(clusters) == 0 {
		return nil
	}

	for _, cluster := range clusters {
		if cluster.Name == name {
			return cluster
		}
	}

	return nil
}

// destroyInfrastructure executes terraform destroy --auto-approve. It destroys whole infrastructure in a project.
func destroyInfrastructure(project *pb.Project) error {
	fmt.Println("Generating templates")
	var backendData Backend
	backendData.ProjectName = project.GetName()
	for _, cluster := range project.Clusters {
		log.Println("Cluster name:", cluster.GetName())
		backendData.ClusterName = cluster.GetName()
		// Creating backend.tf file
		if err := templateGen(templatePath+"/backend.tpl",
			outputPath+"/backend.tf",
			backendData, outputPath); err != nil {
			return err
		}
		// Creating .tf files for providers
		if err := buildNodePools(cluster); err != nil {
			return err
		}
		// Create publicKey file for a cluster
		if err := createKeyFile(cluster.GetPublicKey(), "/public.pem"); err != nil {
			return err
		}
		// Call terraform
		if err := initTerraform(outputPath); err != nil {
			return err
		}

		if err := destroyTerraform(outputPath); err != nil {
			return err
		}

		if err := os.RemoveAll("services/terraformer/server/terraform"); err != nil {
			return err
		}
	}

	return nil
}

// buildNodePools creates .tf files from providers contained in a cluster
func buildNodePools(cluster *pb.Cluster) error {
	for i, nodePool := range cluster.NodePools {
		// HETZNER node pool
		if nodePool.Provider.Name == "hetzner" { // it will return true if hetzner key exists in the credentials map
			log.Println("Cluster provider: ", nodePool.Provider.Name)
			// creating terraform file for a provider
			if err := templateGen(templatePath+"/hetzner.tpl", outputPath+"/hetzner.tf",
				&Data{
					Index:   i,
					Cluster: cluster,
				}, templatePath); err != nil {
				return err
			}
			//nodes = readTerraformOutput(nodes)
		}

		// GCP node pool
		if nodePool.Provider.Name == "gcp" { // it will return true if gcp key exists in the credentials map
			log.Println("Cluster provider: ", nodePool.Provider.Name)
			// creating terraform file for a provider
			if err := templateGen(templatePath+"/gcp.tpl", outputPath+"/gcp.tf",
				&Data{
					Index:   i,
					Cluster: cluster,
				}, templatePath); err != nil {
				return err
			}
			//nodes = readTerraformOutput(nodes)
		}
	}

	return nil
}

// templateGen generates terraform config file from a template .tpl
func templateGen(templatePath string, outputPath string, d interface{}, dirName string) error {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		if err := os.Mkdir(dirName, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create dir: %v", err)
		}
	}

	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to load the template file: %v", err)
	}

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
func initTerraform(fileName string) error {
	// terraform init
	return executeTerraform(exec.Command("terraform", "init"), fileName)
}

// applyTerraform executes terraform apply on a .tf files in a given path
func applyTerraform(fileName string) error {
	// terraform apply --auto-approve
	return executeTerraform(exec.Command("terraform", "apply", "--auto-approve"), fileName)
}

// destroyTerraform executes terraform destroy in a given path
func destroyTerraform(fileName string) error {
	// terraform destroy
	return executeTerraform(exec.Command("terraform", "destroy", "--auto-approve"), fileName)
}

func executeTerraform(cmd *exec.Cmd, fileName string) error {
	cmd.Dir = fileName
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// outputTerraform returns terraform output for a given provider and path in a json format
func outputTerraform(fileName string, provider string) (string, error) {
	cmd := exec.Command("terraform", "output", "-json", provider)
	cmd.Dir = fileName
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
func readOutput(data string) (map[string]map[string]string, error) {
	var result map[string]map[string]string
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result)
	return result, err
}

// fillNodes gets ip addresses from a terraform output
func fillNodes(mOld map[string]*pb.Ip, terraformOutput map[string]map[string]string, nodepool *pb.NodePool) map[string]*pb.Ip {
	mNew := make(map[string]*pb.Ip)
	for key, ip := range terraformOutput["control"] {

		var private = ""
		// If node exist, assign previous private IP
		existingKey, _ := containPublicIP(mOld, ip)
		if existingKey != "" {
			private = mOld[existingKey].Private
		}
		mNew[key] = &pb.Ip{
			Public:       ip,
			Private:      private,
			IsControl:    1,
			Provider:     nodepool.Provider.Name,
			NodepoolName: nodepool.Name,
		}
	}
	for key, ip := range terraformOutput["compute"] {
		var private = ""
		// If node exist, assign previous private IP
		existingKey, _ := containPublicIP(mOld, ip)
		if existingKey != "" {
			private = mOld[existingKey].Private
		}
		mNew[key] = &pb.Ip{
			Public:       ip,
			Private:      private,
			IsControl:    0,
			Provider:     nodepool.Provider.Name,
			NodepoolName: nodepool.Name,
		}
	}
	return mNew
}

func containPublicIP(m map[string]*pb.Ip, ip string) (string, error) {
	for key, ips := range m {
		if ips.Public == ip {
			return key, nil
		}
	}
	return "", fmt.Errorf("ip does not exist")
}

// getProviders returns names of all providers used in a cluster
func getProviders(cluster *pb.Cluster) []string {
	var providers []string
	for _, nodePool := range cluster.NodePools {
		providers = append(providers, nodePool.Provider.Name)
	}
	return providers
}
