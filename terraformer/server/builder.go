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

const hetzner = 0
const gcp = 1

type provider struct {
	name, PublicKey, PrivateKey        string
	using                              bool
	ControlPlane, ComputePlane         int
	ControlPlaneType, ComputePlaneType string
	ProjectName                        string
}

type bucket struct {
	ProjectName string
}

var providers [2]provider

func initializeProviders(project *pb.Project) {
	providers[hetzner] = provider{name: "hetzner", using: false, PublicKey: project.GetPublicKey(), PrivateKey: project.GetPrivateKey(), ProjectName: "testProjectName"}
	providers[gcp] = provider{name: "gcp", using: false, PublicKey: project.GetPublicKey(), PrivateKey: project.GetPrivateKey()}
}

// countNodes is checking how many nodes are for each provider inside cluster and updating []providers
func countNodes(cluster *pb.Cluster) {
	for _, node := range cluster.GetNodes() {
		switch node.GetProvider() {
		case "hetzner":
			providers[hetzner].using = true
			if node.GetIsControlPlane() {
				providers[hetzner].ControlPlane++
				providers[hetzner].ControlPlaneType = node.GetServerType()
			} else {
				providers[hetzner].ComputePlane++
				providers[hetzner].ComputePlaneType = node.GetServerType()
			}
		case "gcp":
			providers[gcp].using = true
			if node.GetIsControlPlane() {
				providers[gcp].ControlPlane++
				providers[gcp].ControlPlaneType = node.GetServerType()
			} else {
				providers[gcp].ComputePlane++
				providers[gcp].ComputePlaneType = node.GetServerType()
			}
		}
	}
}

func createTerraformConfig(templatePath string, outputPath string, s interface{}, fileName string) {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		os.Mkdir(fileName, os.ModePerm)
	}
	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalln("Failed to load the template file", err)
	}
	f, err := os.Create(outputPath)
	if err != nil {
		log.Fatalln("Failed to create the", fileName, "file", err)
	}
	err = tpl.Execute(f, s)
	if err != nil {
		log.Fatalln("Failed to execute the template file", err)
	}
}

// callTerraform function calls terraform and executes a .tf file
func callTerraform(fileName string) {
	fmt.Println("Calling Terraform")
	cmd := exec.Command("terraform", "init", fileName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	cmd = exec.Command("terraform", "apply", "--auto-approve", fileName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func readTerraformOutput(project *pb.Project) {
	f, err := os.Open("./terraform/output")
	if err != nil {
		log.Fatalln("Error while opening output file:", err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	for i := 0; scanner.Scan(); i++ {
		fmt.Println(scanner.Text())
		project.Cluster.Nodes[i].PublicIp = scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		log.Fatalln("Error while reading the output file", err)
	}

}

// generateTemplates is generating terraform files for different providers
func generateTemplates(project *pb.Project) error {
	fmt.Println("Generating provider templates")

	initializeProviders(project)
	countNodes(project.GetCluster())

	// HETZNER
	if providers[hetzner].using {
		createTerraformConfig(
			"./templates/hetzner.tpl",
			"./terraform/hetzner.tf",
			providers[hetzner],
			"terraform",
		) // creating terraform file for a provider
		callTerraform("terraform")
		readTerraformOutput(project)
	}

	return nil
}
