package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
)

func generateKubeConfiguration(templatePath string, outputPath string, d interface{}) {
	if _, err := os.Stat("kubeone"); os.IsNotExist(err) { //this creates a new file if it doesn't exist
		os.Mkdir("kubeone", os.ModePerm)
	}
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
	cmd := exec.Command("kubeone", "apply", "-m", "kubeone.yaml", "-y")
	cmd.Dir = "kubeone" //golang will execute comand from this directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Fprintln(cmd.Stdout)
}

func getKubeconfig() []byte {
	kubeconfig, err := ioutil.ReadFile("./kubeone/cluster-kubeconfig")
	if err != nil {
		log.Fatalln("Error while reading a kubeconfig file", err)
	}
	return kubeconfig
}
