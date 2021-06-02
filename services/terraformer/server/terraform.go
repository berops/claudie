package main

import (
	"bufio"
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
func buildInfrastructure(project *pb.Project) error {
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
		applyTerraform(outputPath)
	}
	return nil
}

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
	}
	return nil
}

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

// applyTerraform function calls terraform init and terraform apply on a .tf file
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
	fmt.Fprintln(cmd.Stdout)
}

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

func readTerraformOutput(nodes []string) []string {
	f, err := os.Open("./terraform/output")
	if err != nil {
		log.Fatalln("Error while opening output file:", err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	//reading terraform output file and filling nodes slice
	for i := 0; scanner.Scan(); i++ {
		fmt.Println(scanner.Text())
		nodes = append(nodes, scanner.Text())
		scanner.Scan()
		fmt.Println(scanner.Text())
		nodes = append(nodes, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatalln("Error while reading the output file", err)
	}

	return nodes
}

// func fillNodes(nodes []string, project *pb.Project) {
// 	j := 0
// 	for i := 0; i < len(nodes); i++ {
// 		project.Cluster.Nodes[j].PublicIp = nodes[i]
// 		fmt.Println(project.Cluster.Nodes[j].PublicIp)
// 		i++
// 		project.Cluster.Nodes[j].Name = nodes[i]
// 		fmt.Println(project.Cluster.Nodes[j].Name)
// 		j++
// 	}
// }
