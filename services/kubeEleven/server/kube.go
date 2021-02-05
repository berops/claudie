package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/Berops/platform/proto/pb"
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
	cmd.Dir = "kubeone" //golang will execute command from this directory
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Fprintln(cmd.Stdout)
}

// getKubeconfig reads kubeconfig from a file and returns it
func getKubeconfig() []byte {
	kubeconfig, err := ioutil.ReadFile("./kubeone/cluster-kubeconfig")
	if err != nil {
		log.Fatalln("Error while reading a kubeconfig file", err)
	}
	return kubeconfig
}

func removeNode(project *pb.Project) {
	//kubectl get nodes
	cmd := "kubectl get nodes --kubeconfig <(echo '" + string(project.GetCluster().GetKubeconfig()) + "') -owide -- | awk '{if(NR>1)print $1}'"
	output, err := exec.Command("bash", "-c", cmd).CombinedOutput()

	if err != nil {
		log.Fatalln("Error while executing get nodes:", err)
	}
	//split output into slice
	clusterNodes := strings.Fields(string(output))

	//Check which node is redundant and create slice of diffNodes
	var projectNodes []string
	for _, node := range project.GetCluster().GetNodes() {
		projectNodes = append(projectNodes, node.GetName())
	}
	diffNodes := difference(clusterNodes, projectNodes)
	fmt.Println(diffNodes)

	if diffNodes != nil {
		//kubectl drain <node-name> --ignore-daemonsets --delete-local-data ,all diffNodes
		for _, node := range diffNodes {
			fmt.Println("kubectl drain " + node + " --ignore-daemonsets --delete-local-data")
			cmd := "kubectl drain " + node + " --ignore-daemonsets --delete-local-data --kubeconfig <(echo '" + string(project.GetCluster().GetKubeconfig()) + "')"
			_, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				log.Fatalln("Error while draining node "+node+":", err)
			}
		}

		//kubectl delete node <node-name>
		for _, node := range diffNodes {
			fmt.Println("kubectl delete node " + node)
			cmd := "kubectl delete node " + node + " --kubeconfig <(echo '" + string(project.GetCluster().GetKubeconfig()) + "')"
			_, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				log.Fatalln("Error while deleting node "+node+":", err)
			}
		}
	}
	fmt.Println("I GOT HEREEEEEE!!!")
}

func difference(slice1 []string, slice2 []string) []string {
	var diff []string

	// Loop two times, first to find slice1 strings not in slice2,
	// second loop to find slice2 strings not in slice1
	for i := 0; i < 2; i++ {
		for _, s1 := range slice1 {
			found := false
			for _, s2 := range slice2 {
				if s1 == s2 {
					found = true
					break
				}
			}
			// String not found. We add it to return slice
			if !found {
				diff = append(diff, s1)
			}
		}
		// Swap the slices, only if it was the first loop
		if i == 0 {
			slice1, slice2 = slice2, slice1
		}
	}

	return diff
}
