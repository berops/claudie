package testingframework

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
)

const (
	maxLonghornCheck = 30 * 60 // max allowed time for pods of longhorn-system to be ready [seconds]
	sleepSecPods     = 20      // seconds for one cycle of longhorn checks (the node and pod checks)
)

type KubectlOutputJSON struct {
	APIVersion string                   `json:"apiVersion"`
	Items      []map[string]interface{} `json:"items"`
	Kind       string                   `json:"kind"`
	Metadata   map[string]interface{}   `json:"metadata"`
}

// testLonghornDeployment function will perform actions needed to confirm that longhorn has been successfully deployed in the cluster
func testLonghornDeployment(ctx context.Context, config *pb.Config) error {
	//start longhorn testing
	clusters := config.CurrentState.Clusters
	for _, cluster := range clusters {
		// check number of nodes in nodes.longhorn.io

		kubectl := kubectl.Kubectl{Kubeconfig: cluster.Kubeconfig}
		err := checkLonghornNodes(ctx, cluster, kubectl)
		if err != nil {
			return fmt.Errorf("error while checking the nodes.longhorn.io in cluster %s : %w", cluster.ClusterInfo.Name, err)
		}
		// check if all pods from longhorn-system are ready
		err = checkLonghornPods(ctx, cluster.ClusterInfo.Name, kubectl)
		if err != nil {
			return fmt.Errorf("error while checking if all pods from longhorn-system are ready in cluster %s: %w", cluster.ClusterInfo.Name, err)
		}
	}
	return nil
}

// checkLonghornNodes will check if the count of nodes.longhorn.io is same as number of schedulable nodes
func checkLonghornNodes(ctx context.Context, cluster *pb.K8Scluster, kubectl kubectl.Kubectl) error {
	readyCheck := 0
	workerCount := 0
	//count the worker nodes
	for _, nodepool := range cluster.ClusterInfo.NodePools {
		if !nodepool.IsControl {
			workerCount += int(nodepool.Count)
		}
	}
	// give them time of maxLonghornCheck seconds to be scheduled
	for {
		select {
		case <-ctx.Done():
			return errInterrupt
		default:
			out, err := kubectl.KubectlGet("nodes.longhorn.io -n longhorn-system -o json", "")
			if err != nil {
				return fmt.Errorf("error while getting the nodes.longhorn.io in cluster %s : %w", cluster.ClusterInfo.Name, err)
			}
			nodeCountFound, err := parseNodesOutput(out)
			if err != nil {
				return fmt.Errorf("error while checking the kubectl output for  nodes.longhorn.io in cluster  %s : %w", cluster.ClusterInfo.Name, err)
			}
			// the number of worker nodes should be equal to number of scheduled nodes in longhorn
			// NOTE: by default, master nodes will not be used to schedule pods, however, if this changes the condition will be broken
			if nodeCountFound == workerCount {
				return nil
			}
			readyCheck += sleepSecPods
			if readyCheck >= maxLonghornCheck {
				return fmt.Errorf("the count of schedulable nodes (%d) is not equal to nodes.longhorn.io (%d) in cluster %s", workerCount, nodeCountFound, cluster.ClusterInfo.Name)
			}
			time.Sleep(time.Duration(sleepSecPods) * time.Second)
			log.Info().Msgf("Waiting for nodes.longhorn.io to be initialized in cluster %s... [ %ds elapsed ]", cluster.ClusterInfo.Name, readyCheck)
		}
	}
}

// checkLonghornPods will check if the pods in longhorn-system namespace are in ready state
func checkLonghornPods(ctx context.Context, clusterName string, kubectl kubectl.Kubectl) error {
	readyCheck := 0
	for {
		select {
		case <-ctx.Done():
			return errInterrupt
		default:
			out, err := kubectl.KubectlGet("pods -o json", "longhorn-system")
			if err != nil {
				return fmt.Errorf("error while getting the status of the pods in longhorn-system in cluster %s : %w", clusterName, err)
			}
			ready, err := parsePodsOutput(out)
			if err != nil {
				log.Warn().Msgf("Error while parsing kubectl output for longhorn pods in %s : %v", clusterName, err)
			}
			// if some are not ready, wait sleepSecPods seconds
			if !ready {
				readyCheck += sleepSecPods
			} else {
				return nil
			}
			// Timeout check
			if readyCheck >= maxLonghornCheck {
				return fmt.Errorf("pods in longhorn-system took too long to initialize in cluster %s", clusterName)
			}
			time.Sleep(time.Duration(sleepSecPods) * time.Second)
			log.Info().Msgf("Waiting for pods from longhorn-system namespace in cluster %s to be in ready state... [ %ds elapsed ]", clusterName, readyCheck)
		}
	}
}

// function will parse kubectl json output regarding the longhorn nodes
// returns true if every pod is ready, false otherwise
func parseNodesOutput(out []byte) (int, error) {
	// parse output
	var parsedJSON KubectlOutputJSON
	err := json.Unmarshal(out, &parsedJSON)
	if err != nil {
		return -1, fmt.Errorf("error while unmarshalling output data : %w", err)
	}
	return len(parsedJSON.Items), nil
}

// function will parse kubectl json output regarding the longhorn pods
// returns true if every pod is ready, false otherwise
func parsePodsOutput(out []byte) (bool, error) {
	// parse output
	var parsedJSON KubectlOutputJSON
	err := json.Unmarshal(out, &parsedJSON)
	if err != nil {
		return false, fmt.Errorf("error while unmarshalling output data : %w", err)
	}
	// iterate over all returned items
	for _, item := range parsedJSON.Items {
		if item == nil {
			return false, nil
		}
		// get status field
		statusField := item["status"]
		if statusField == nil {
			return false, nil
		}
		status := statusField.(map[string]interface{})
		// get container statuses
		containerStatusesField := status["containerStatuses"]
		if containerStatusesField == nil {
			return false, nil
		}
		containerStatuses := containerStatusesField.([]interface{})
		// check all container statuses if they are ready
		for _, conStat := range containerStatuses {
			if conStat == nil {
				return false, nil
			}
			readyField := conStat.(map[string]interface{})
			if readyField == nil {
				return false, nil
			}
			ready := readyField["ready"].(bool)
			// if not ready, return false
			if !ready {
				log.Info().Msgf("Container %s is not ready yet...", conStat.(map[string]interface{})["name"].(string))
				return false, nil
			}
		}
	}
	return true, nil
}
