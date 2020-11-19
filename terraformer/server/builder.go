package main

import (
	"fmt"
	"log"
	"os"
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
}

var providers [2]provider

func initializeProviders(project *pb.Project) {
	providers[hetzner] = provider{name: "hetzner", using: false, PublicKey: project.GetPublicKey(), PrivateKey: project.GetPrivateKey()}
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

func createTemplateFile(templatePath string, outputPath string, p provider) {
	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalln("Failed to load template file", err)
	}
	f, err := os.Create(outputPath)
	if err != nil {
		log.Fatalln("Failed to create a terraform file", err)
	}
	err = tpl.Execute(f, p)
	if err != nil {
		log.Fatalln("Failed to execute template file", err)
	}
}

// generateTemplates is generating terraform files for different providers
func generateTemplates(project *pb.Project) error {
	fmt.Println("Generating provider templates")
	initializeProviders(project)
	countNodes(project.GetCluster())

	templatePath := "./templates/hetzner.gotf"
	outputPath := "./terraform/hetzner.tf"

	// HETZNER
	if providers[hetzner].using {
		createTemplateFile(templatePath, outputPath, providers[hetzner])
	}

	return nil
}
