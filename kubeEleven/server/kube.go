package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
)

func generateKubeConfiguration(templatePath string, outputPath string, d interface{}) {
	tpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalln("Failed to load the template file", err)
	}
	f, err := os.Create(outputPath)
	if err != nil {
		log.Fatalln("Failed to create the manifest file", err)
	}
	err = tpl.Execute(f, d)
	if err != nil {
		log.Fatalln("Failed to execute the template file", err)
	}
}

func runKubeOne() {
	fmt.Println("Running KubeOne")
	cmd := exec.Command("kubeone", "apply", "kubeone.yaml")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Fprintln(cmd.Stdout)
}

