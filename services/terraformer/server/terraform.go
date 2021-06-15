package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// buildInfrastructure is generating terraform files for different providers and calling terraform
func buildInfrastructure(desiredState *pb.Project) (*pb.Project, error) {
	fmt.Println("Generating templates")
	var backendData Backend
	backendData.ProjectName = desiredState.GetName()
	for _, cluster := range desiredState.Clusters {
		providers := getProviders(cluster)
		log.Println("Cluster name:", cluster.GetName())
		backendData.ClusterName = cluster.GetName()
		// Creating backend.tf file from the template
		templateGen(templatePath+"/backend.tpl", outputPath+"/backend.tf", backendData, outputPath)
		// Creating .tf files for providers from templates
		buildNodePools(cluster)
		// Call terraform init and apply
		initTerraform(outputPath)
		applyTerraform(outputPath)
		// Fill public ip addresses
		m := make(map[string]*pb.Ip)
		for _, provider := range providers {
			output, err := outputTerraform(outputPath, provider)
			if err != nil {
				log.Fatalln(err)
			}
			fillNodes(m, readOutput(output))
		}
		cluster.Ips = m
		// Clean after Terraform. Remove tmp terraform dir.
		err := os.RemoveAll("services/terraformer/server/terraform")
		if err != nil {
			log.Fatal(err)
		}
	}
	for _, m := range desiredState.Clusters {
		log.Println(m.Ips)
	}

	return desiredState, nil
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
		templateGen(templatePath+"/backend.tpl", outputPath+"/backend.tf", backendData, outputPath)
		// Creating .tf files for providers
		buildNodePools(cluster)
		// Call terraform
		initTerraform(outputPath)
		destroyTerraform(outputPath)
		err := os.RemoveAll("services/terraformer/server/terraform")
		if err != nil {
			log.Fatal(err)
		}
	}
	return nil
}

// buildNodePools creates .tf files from providers contained in a cluster
func buildNodePools(cluster *pb.Cluster) {
	for i, nodePool := range cluster.NodePools {

		// HETZNER node pool
		if nodePool.Provider.Name == "hetzner" { // it will return true if hetzner key exists in the credentials map
			log.Println("Cluster provider: ", nodePool.Provider.Name)
			// creating terraform file for a provider
			templateGen(templatePath+"/hetzner.tpl", outputPath+"/hetzner.tf",
				&Data{
					Index:   i,
					Cluster: cluster,
				}, templatePath)
			//nodes = readTerraformOutput(nodes)
		}

		// GCP node pool
		if nodePool.Provider.Name == "gcp" { // it will return true if gcp key exists in the credentials map
			log.Println("Cluster provider: ", nodePool.Provider.Name)
			// creating terraform file for a provider
			templateGen(templatePath+"/gcp.tpl", outputPath+"/gcp.tf",
				&Data{
					Index:   i,
					Cluster: cluster,
				}, templatePath)
			//nodes = readTerraformOutput(nodes)
		}
	}
}

// templateGen generates terraform config file from a template .tpl
func templateGen(templatePath string, outputPath string, d interface{}, dirName string) {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		os.Mkdir(dirName, os.ModePerm)
	}
	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalln("Failed to load the template file", err)
	}
	f, err := os.Create(outputPath)
	if err != nil {
		log.Fatalln("Failed to create the", dirName, "file", err)
	}
	err = tpl.Execute(f, d)
	if err != nil {
		log.Fatalln("Failed to execute the template file", err)
	}
}

// initTerraform executes terraform init in a given path
func initTerraform(fileName string) {
	// terraform init
	cmd := exec.Command("terraform", "init")
	cmd.Dir = fileName
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// applyTerraform executes terraform terraform apply on a .tf files in a given path
func applyTerraform(fileName string) {
	// terraform apply --auto-approve
	cmd := exec.Command("terraform", "apply", "--auto-approve")
	cmd.Dir = fileName
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// destroyTerraform executes terraform destroy in a given path
func destroyTerraform(fileName string) {
	// terraform destroy
	cmd := exec.Command("terraform", "destroy", "--auto-approve")
	cmd.Dir = fileName
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
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
		log.Fatal(err)
	}
	return outb.String(), nil
}

// readOutput reads json output format from terraform and unmarshal it into map[string]map[string]string readable by GO
func readOutput(data string) map[string]map[string]string {
	var result map[string]map[string]string
	// Unmarshal or Decode the JSON to the interface.
	err := json.Unmarshal([]byte(data), &result)
	if err != nil {
		log.Fatalln(err)
	}
	return result
}

// fillNodes gets ip addresses from a terraform output
func fillNodes(m map[string]*pb.Ip, terraformOutput map[string]map[string]string) {
	for key, element := range terraformOutput["control"] {
		log.Println("Key:", key, "=>", "Element:", element)
		m[key] = &pb.Ip{Public: element}
	}
	for key, element := range terraformOutput["compute"] {
		log.Println("Key:", key, "=>", "Element:", element)
		m[key] = &pb.Ip{Public: element}
	}
}

// getProviders returns names of all providers used in a cluster
func getProviders(cluster *pb.Cluster) []string {
	var providers []string
	for _, nodePool := range cluster.NodePools {
		providers = append(providers, nodePool.Provider.Name)
	}
	return providers
}
