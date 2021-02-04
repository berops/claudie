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

func readTerraformOutput(project *pb.Project) {
	f, err := os.Open("./terraform/output")
	if err != nil {
		log.Fatalln("Error while opening output file:", err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	//TEMPORARY SOLUTION reading terraform output file and filling cluster with ip addresses
	for i := 0; scanner.Scan(); i++ {
		fmt.Println(scanner.Text())
		for j := i; j < len(project.Cluster.Nodes); j++ {
			if project.Cluster.Nodes[j].PublicIp == "" {
				project.Cluster.Nodes[j].PublicIp = scanner.Text()
				scanner.Scan()
				fmt.Println(scanner.Text())
				project.Cluster.Nodes[j].Name = scanner.Text()
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalln("Error while reading the output file", err)
	}

	fmt.Println(project)
}

// buildTerraform is generating terraform files for different providers and calling terraform
func buildTerraform(project *pb.Project) error {
	fmt.Println("Generating templates")

	createTerraformConfig("./templates/backend.tpl", "./terraform/backend.tf", project, "terraform")
	// HETZNER
	if project.Cluster.Providers["hetzner"].IsInUse {
		createTerraformConfig("./templates/hetzner.tpl", "./terraform/hetzner.tf", project, "terraform") // creating terraform file for a provider
		callTerraform("terraform")
		readTerraformOutput(project)
	}
	// GCP
	if project.Cluster.Providers["gcp"].IsInUse {
		createTerraformConfig("./templates/gcp.tpl", "./terraform/gcp.tf", project, "terraform") // creating terraform file for a provider
		callTerraform("terraform")
		readTerraformOutput(project)
	}

	return nil
}
