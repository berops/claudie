package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
)

type etcdPodInfo struct {
	nodeName   string
	memberHash string
}

var (
	exportCmd = `export ETCDCTL_API=3 && 
		export ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt && 
		export ETCDCTL_CERT=/etc/kubernetes/pki/etcd/healthcheck-client.crt && 
		export ETCDCTL_KEY=/etc/kubernetes/pki/etcd/healthcheck-client.key`
	getEtcdPodsCmd = "get pods -n kube-system --no-headers -o custom-columns=\":metadata.name\" | grep etcd"
)

func deleteNodes(config *pb.Config, toDelete map[string]*nodesToDelete) (*pb.Config, error) {
	for _, cluster := range config.CurrentState.Clusters {
		var nodesToDelete []string
		var etcdToDelete []string
		del := toDelete[cluster.ClusterInfo.Name]
		for _, nodepool := range cluster.ClusterInfo.NodePools {
			for i := len(nodepool.Nodes) - 1; i >= 0; i-- {
				val, ok := del.nodes[nodepool.Name]
				if val.Count > 0 && ok {
					if nodepool.Nodes[i].NodeType > pb.NodeType_worker {
						val.Count--
						nodesToDelete = append(nodesToDelete, nodepool.Nodes[i].GetName())
						etcdToDelete = append(etcdToDelete, nodepool.Nodes[i].GetName())
						log.Info().Msgf("Choosing Master node %s, with public IP %s, private IP %s for deletion\n", nodepool.Nodes[i].GetName(), nodepool.Nodes[i].GetPublic(), nodepool.Nodes[i].GetPrivate())
						continue
					}
					if nodepool.Nodes[i].NodeType == pb.NodeType_worker {
						val.Count--
						nodesToDelete = append(nodesToDelete, nodepool.Nodes[i].GetName())
						log.Info().Msgf("Choosing Worker node %s, with public IP %s, private IP %s for deletion\n", nodepool.Nodes[i].GetName(), nodepool.Nodes[i].GetPublic(), nodepool.Nodes[i].GetPrivate())
						continue
					}
				}
			}
		}

		// Delete nodes from an etcd
		if len(etcdToDelete) > 0 {
			err := deleteFromEtcd(cluster, etcdToDelete)
			if err != nil {
				return nil, err
			}
		}
		// Delete nodes from a cluster
		err := deleteNodesByName(cluster, nodesToDelete)
		if err != nil {
			return nil, err
		}

		// Delete nodes from a current state Ips map
		for _, nodeName := range nodesToDelete {
			for _, nodepool := range cluster.ClusterInfo.NodePools {
				for idx, node := range nodepool.Nodes {
					if node.GetName() == nodeName {
						nodepool.Count = nodepool.Count - 1
						nodepool.Nodes = append(nodepool.Nodes[:idx], nodepool.Nodes[idx+1:]...)
					}
				}
			}
		}
	}
	return config, nil
}

// deleteNodesByName checks if there is any difference in nodes between a desired state cluster and a running cluster
func deleteNodesByName(cluster *pb.K8Scluster, nodesToDelete []string) error {
	// get real node names
	realNodeNames, err := kcGetNodeNames(cluster.Kubeconfig)
	if err != nil {
		return fmt.Errorf("error while getting nodes from cluster %s : %v", cluster.ClusterInfo.Name, err)
	}
	// parse list of pods returned
	for _, nodeName := range nodesToDelete {
		realNodeName := utils.FindName(realNodeNames, nodeName)
		if realNodeName != "" {
			//kubectl drain <node-name> --ignore-daemonsets --delete-local-data ,all diffNodes
			err := kcDrainNode(realNodeName, cluster.Kubeconfig)
			if err != nil {
				return fmt.Errorf("error while deleting node %s : %v", nodeName, err)
			}
			//kubectl delete node <node-name>
			err = kcDeleteNode(realNodeName, cluster.Kubeconfig)
			if err != nil {
				return fmt.Errorf("error while deleting node %s : %v", nodeName, err)
			}
		} else {
			log.Error().Msgf("Node name that contains \"%s\" no found ", nodeName)
			return fmt.Errorf("no node with name %s found ", nodeName)
		}
	}
	return nil
}

func deleteFromEtcd(cluster *pb.K8Scluster, mastersToDelete []string) error {
	mainMasterNode := getMainMaster(cluster)
	if mainMasterNode == nil {
		log.Error().Msg("APIEndpoint node not found")
		return fmt.Errorf("failed to find any node with IsControl value as 2 in cluster %s", cluster.ClusterInfo.Name)
	}
	//get etcd pods
	etcdPods, err := getEtcdPods(mainMasterNode, cluster)
	if err != nil {
		log.Error().Msgf("Cannot find etcd pods in cluster %s : %v", cluster.ClusterInfo.Name, err)
		return fmt.Errorf("cannot find etcd pods in cluster %s  : %v", cluster.ClusterInfo.Name, err)
	}
	kcExecEtcdCmd := fmt.Sprintf("kubectl --kubeconfig <(echo '%s') -n kube-system exec -i %s -- /bin/sh -c ",
		cluster.GetKubeconfig(), etcdPods[0])
	//get etcd members
	etcdMembersString, err := getEtcdMembers(cluster, kcExecEtcdCmd)
	if err != nil {
		log.Error().Msgf("Cannot find etcd members in cluster %s : %v", cluster.ClusterInfo.Name, err)
		return fmt.Errorf("cannot find etcd members in cluster %s : %v", cluster.ClusterInfo.Name, err)
	}
	//get pod info, like name of a node where pod is deployed and etcd member hash
	etcdPodInfos := getEtcdPodInfo(etcdMembersString)
	// Remove etcd members that are in mastersToDelete, you need to know an etcd node hash to be able to remove a member
	for _, nodeName := range mastersToDelete {
		for _, etcdPodInfo := range etcdPodInfos {
			if nodeName == etcdPodInfo.nodeName {
				log.Info().Msgf("Removing node %s, with etcd member hash %s \n", etcdPodInfo.nodeName, etcdPodInfo.memberHash)
				cmd := fmt.Sprintf("%s \" %s && etcdctl member remove %s \"", kcExecEtcdCmd, exportCmd, etcdPodInfo.memberHash)
				err := exec.Command("bash", "-c", cmd).Run()
				if err != nil {
					log.Error().Msgf("Error while etcdctl member remove: %v", err)
					return err
				}
			}
		}
	}
	return nil
}

func getEtcdPods(master *pb.Node, cluster *pb.K8Scluster) ([]string, error) {
	// get etcd pods name
	podsQueryCmd := fmt.Sprintf("kubectl --kubeconfig <(echo \"%s\") %s-%s", cluster.GetKubeconfig(), getEtcdPodsCmd, master.Name)
	output, err := exec.Command("bash", "-c", podsQueryCmd).CombinedOutput()
	if err != nil {
		log.Error().Msgf("Failed to get list of pods with name: etcd-%s", master.Name)
		return nil, err
	}
	return strings.Split(string(output), "\n"), nil
}

func getEtcdMembers(cluster *pb.K8Scluster, kcExecEtcdCmd string) ([]string, error) {
	// Execute into the working etcd container and setup client TLS authentication in order to be able to communicate
	// with etcd and get output of all etcd members
	cmd := fmt.Sprintf("%s \" %s && etcdctl member list \"", kcExecEtcdCmd, exportCmd)
	output, err := exec.Command("bash", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error().Msgf("Error while executing command %s in a working etcd container: %v", cmd, err)
		log.Error().Msgf("prepCmd was %s", kcExecEtcdCmd)
		return nil, err
	}
	// Example output:
	// 3ea84f69be8336f3, started, test2-cluster-name1-hetzner-control-2, https://192.168.2.2:2380, https://192.168.2.2:2379, false
	// 56c921bc723229ec, started, test2-cluster-name1-hetzner-control-1, https://192.168.2.1:2380, https://192.168.2.1:2379, false
	// Convert output into []string, each line of output is a separate string
	etcdStrings := strings.Split(string(output), "\n")
	//delete last entry - empty \n
	if len(etcdStrings) > 0 {
		etcdStrings = etcdStrings[:len(etcdStrings)-1]
	}
	return etcdStrings, nil
}

func getEtcdPodInfo(etcdMembersString []string) []etcdPodInfo {
	var etcdPodInfos []etcdPodInfo
	for _, etcdString := range etcdMembersString {
		etcdStringTokenized := strings.Split(etcdString, ", ")
		if len(etcdStringTokenized) > 0 {
			temp := etcdPodInfo{etcdStringTokenized[2] /*name*/, etcdStringTokenized[0] /*hash*/}
			etcdPodInfos = append(etcdPodInfos, temp)
		}
	}
	return etcdPodInfos
}

func getMainMaster(cluster *pb.K8Scluster) *pb.Node {
	for _, nodepool := range cluster.ClusterInfo.GetNodePools() {
		for _, node := range nodepool.Nodes {
			if node.NodeType == pb.NodeType_apiEndpoint {
				return node
			}
		}
	}
	return nil
}

func kcDrainNode(nodeName, kubeconfig string) error {
	log.Info().Msgf("kubectl drain %s --ignore-daemonsets --delete-local-data", nodeName)
	cmd := fmt.Sprintf("kubectl drain %s --ignore-daemonsets --delete-local-data --kubeconfig <(echo '%s')", nodeName, kubeconfig)
	err := exec.Command("bash", "-c", cmd).Run()
	if err != nil {
		log.Error().Msgf("Error while draining node %s : %v", nodeName, err)
		return err
	}
	return nil
}

func kcDeleteNode(nodeName, kubeconfig string) error {
	log.Info().Msgf("kubectl delete node %s" + nodeName)
	cmd := fmt.Sprintf("kubectl delete node %s --kubeconfig <(echo '%s')", nodeName, kubeconfig)
	err := exec.Command("bash", "-c", cmd).Run()
	if err != nil {
		log.Error().Msgf("Error while deleting node %s : %v", nodeName, err)
		return err
	}
	return nil
}

func kcGetNodeNames(kubeconfig string) ([]string, error) {
	nodesQueryCmd := fmt.Sprintf("kubectl --kubeconfig <(echo \"%s\") get nodes -n kube-system --no-headers -o custom-columns=\":metadata.name\" ", kubeconfig)
	output, err := exec.Command("bash", "-c", nodesQueryCmd).CombinedOutput()
	if err != nil {
		log.Error().Msgf("Failed to get list of nodes ")
		return nil, err
	}
	return strings.Split(string(output), "\n"), nil
}
