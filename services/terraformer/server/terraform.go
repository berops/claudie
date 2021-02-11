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

// createTerraformConfig generates terraform config file
func createTerraformConfig(templatePath string, outputPath string, d interface{}, dirName string) {
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
	fmt.Fprintln(cmd.Stdout)
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

func fillNodes(nodes []string, project *pb.Project) {
	j := 0
	for i := 0; i < len(nodes); i++ {
		project.Cluster.Nodes[j].PublicIp = nodes[i]
		fmt.Println(project.Cluster.Nodes[j].PublicIp)
		i++
		project.Cluster.Nodes[j].Name = nodes[i]
		fmt.Println(project.Cluster.Nodes[j].Name)
		j++
	}
}

// buildTerraform is generating terraform files for different providers and calling terraform
func buildTerraform(project *pb.Project) error {
	fmt.Println("Generating templates")
	var nodes []string

	createTerraformConfig("./templates/backend.tpl", "./terraform/backend.tf", project, "terraform")
	// HETZNER
	if project.Cluster.Providers["hetzner"].IsInUse {
		createTerraformConfig("./templates/hetzner.tpl", "./terraform/hetzner.tf", project, "terraform") // creating terraform file for a provider
		callTerraform("terraform")
		nodes = readTerraformOutput(nodes)
	}
	// GCP
	if project.Cluster.Providers["gcp"].IsInUse {
		createTerraformConfig("./templates/gcp.tpl", "./terraform/gcp.tf", project, "terraform") // creating terraform file for a provider
		callTerraform("terraform")
		nodes = readTerraformOutput(nodes)
	}
	fmt.Println("NODEEEEEEEEEEES:", nodes)
	fillNodes(nodes, project)

	return nil
}
