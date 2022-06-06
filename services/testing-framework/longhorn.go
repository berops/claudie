package testingframework

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Berops/platform/proto/pb"
	"github.com/rs/zerolog/log"
)

const (
	maxLonghornCheck = 240 // max allowed time for pods of longhorn-system to be ready [seconds]
	sleepSecPods     = 10  // seconds for one cycle of longhorn checks (the node and pod checks)
)

// checkLonghornNodes will check if the count of nodes.longhorn.io is same as number of schedulable nodes
func checkLonghornNodes(cluster *pb.K8Scluster) error {
	command := fmt.Sprintf("kubectl get nodes.longhorn.io -A -o json --kubeconfig <(echo '%s') | jq -r '.items | length '", cluster.Kubeconfig)
	allNodesFound := false
	readyCheck := 0
	workerCount := 0
	count := "" //in order to save last value for error message, the var is defined here
	//count the worker nodes
	for _, nodepool := range cluster.ClusterInfo.NodePools {
		if !nodepool.IsControl {
			workerCount += int(nodepool.Count)
		}
	}
	// give them time of maxLonghornCheck seconds to be scheduled
	for readyCheck < maxLonghornCheck {
		cmd := exec.Command("/bin/bash", "-c", command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			emsg := fmt.Sprintf("error while getting the nodes.longhorn.io count in cluster %s : %v", cluster.ClusterInfo.Name, err)
			return fmt.Errorf(emsg)
		}
		//trim the whitespaces
		count = strings.Trim(string(out), "\t\n ")
		// the number of worker nodes should be equal to number of scheduled nodes in longhorn
		// NOTE: by default, master nodes will not be used to schedule pods, however, if this changes the condition will be broken
		if count != fmt.Sprint(workerCount) {
			readyCheck += sleepSecPods
		} else {
			allNodesFound = true
			break
		}
		time.Sleep(time.Duration(sleepSecPods) * time.Second)
		log.Info().Msgf("Waiting for nodes.longhorn.io to be initialized in cluster %s... [ %ds elapsed ]", cluster.ClusterInfo.Name, readyCheck)
	}
	if !allNodesFound {
		return fmt.Errorf(fmt.Sprintf("the count of schedulable nodes (%d) is not equal to nodes.longhorn.io (%s) in cluster %s", workerCount, count, cluster.ClusterInfo.Name))
	}
	return nil
}

// checkLonghornPods will check if the pods in longhorn-system namespace are in ready state
func checkLonghornPods(config, clusterName string) error {
	command := fmt.Sprintf("kubectl get pods -n longhorn-system -o json --kubeconfig <(echo '%s') | jq -r '.items[] | .status.containerStatuses[].ready'", config)
	readyCheck := 0
	allPodsReady := false
	// give them time of maxLonghornCheck seconds to be scheduled
	for readyCheck < maxLonghornCheck {
		cmd := exec.Command("/bin/bash", "-c", command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf(fmt.Sprintf("error while getting the status of the pods in longhorn-system in cluster %s : %v", clusterName, err))
		}
		// if some are not ready, wait sleepSecPods seconds
		if strings.Contains(string(out), "false") {
			readyCheck += sleepSecPods
		} else {
			allPodsReady = true
			break
		}
		time.Sleep(time.Duration(sleepSecPods) * time.Second)
		log.Info().Msgf("Waiting for pods from longhorn-system namespace in cluster %s to be in ready state... [ %ds elapsed ]", clusterName, readyCheck)
	}
	if !allPodsReady {
		return fmt.Errorf("pods in longhorn-system took too long to initialize in cluster %s", clusterName)
	}
	return nil
}

// testLonghornDeployment function will perform actions needed to confirm that longhorn has been successfully deployed in the cluster
func testLonghornDeployment(config *pb.GetConfigFromDBResponse, done chan string) error {
	//start longhorn testing
	clusters := config.Config.CurrentState.Clusters
	for _, cluster := range clusters {
		// check number of nodes in nodes.longhorn.io
		err := checkLonghornNodes(cluster)
		if err != nil {
			return fmt.Errorf("error while checking the nodes.longhorn.io : %v", err)

		}
		// check if all pods from longhorn-system are ready
		err = checkLonghornPods(cluster.Kubeconfig, cluster.ClusterInfo.Name)
		if err != nil {
			return fmt.Errorf("error while checking if all pods from longhorn-system are ready : %v", err)
		}
	}
	return nil
}
