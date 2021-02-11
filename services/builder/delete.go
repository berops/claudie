package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/Berops/platform/proto/pb"
)

func deleteNodes(project *pb.Project) error {
	//kubectl get nodes
	cmd := "kubectl get nodes --kubeconfig <(echo '" + string(project.GetCluster().GetKubeconfig()) + "') -owide -- | awk '{if(NR>1)print $1}'"
	output, err := exec.Command("bash", "-c", cmd).CombinedOutput()

	if err != nil {
		log.Println("Error while executing get nodes:", err)
		return err
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
				log.Println("Error while draining node "+node+":", err)
				return err
			}
		}

		//kubectl delete node <node-name>
		for _, node := range diffNodes {
			fmt.Println("kubectl delete node " + node)
			cmd := "kubectl delete node " + node + " --kubeconfig <(echo '" + string(project.GetCluster().GetKubeconfig()) + "')"
			_, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				log.Println("Error while deleting node "+node+":", err)
				return err
			}
		}
	}

	return err
}

// difference returns slice of strings which are in the first slice but not in the second slice
func difference(slice1 []string, slice2 []string) []string {
	var diff []string

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

	return diff
}
