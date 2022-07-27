package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Berops/platform/proto/pb"
	"github.com/rs/zerolog/log"
)

type etcdNodeInfo struct {
	nodeName string
	nodeHash string
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
			err := deleteEtcd(cluster, etcdToDelete)
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

	// get node name
	nodesQueryCmd := fmt.Sprintf("kubectl --kubeconfig <(echo \"%s\") get nodes -n kube-system --no-headers -o custom-columns=\":metadata.name\" ", cluster.GetKubeconfig())
	output, err := exec.Command("bash", "-c", nodesQueryCmd).CombinedOutput()
	if err != nil {
		log.Error().Msgf("Failed to get list of nodes ")
		return err
	}

	// parse list of pods returned
	nodeNames := strings.Split(string(output), "\n")

	//kubectl drain <node-name> --ignore-daemonsets --delete-local-data ,all diffNodes
	for _, nodeNameSubString := range nodesToDelete {
		nodeName, found := searchNodeNames(nodeNames, nodeNameSubString)
		if found {
			log.Info().Msgf("kubectl drain %s --ignore-daemonsets --delete-local-data", nodeName)
			cmd := fmt.Sprintf("kubectl drain %s --ignore-daemonsets --delete-local-data --kubeconfig <(echo '%s')", nodeName, cluster.GetKubeconfig())
			res, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				log.Error().Msgf("Error while draining node %s : %v", nodeName, err)
				log.Error().Bytes("result", res)
				return err
			}
		} else {
			log.Error().Msgf("Node name that contains \"%s\" no found ", nodeNameSubString)
			return fmt.Errorf("no node with name %s found ", nodeNameSubString)
		}

	}

	//kubectl delete node <node-name>
	for _, nodeNameSubString := range nodesToDelete {
		nodeName, found := searchNodeNames(nodeNames, nodeNameSubString)

		if found {
			log.Info().Msgf("kubectl delete node %s" + nodeName)
			cmd := fmt.Sprintf("kubectl delete node %s --kubeconfig <(echo '%s')", nodeName, cluster.GetKubeconfig())
			_, err := exec.Command("bash", "-c", cmd).CombinedOutput()
			if err != nil {
				log.Error().Msgf("Error while deleting node %s : %v", nodeName, err)
				return err
			}
		} else {
			log.Error().Msgf("Node name that contains \"%s\" no found ", nodeNameSubString)
			return fmt.Errorf("no node with name %s found ", nodeNameSubString)
		}
	}
	return nil
}

func deleteEtcd(cluster *pb.K8Scluster, etcdToDelete []string) error {
	mainMasterNode := getMainMaster(cluster)
	if mainMasterNode == nil {
		log.Error().Msg("APIEndpoint node not found")
		return fmt.Errorf("failed to find any node with IsControl value as 2")
	}
	etcdPods, err := getEtcdPods(mainMasterNode, cluster)
	if err != nil {
		log.Error().Msgf("Cannot find etcd pods in cluster : %v", err)
		return fmt.Errorf("cannot find etcd pods in cluster : %v", err)
	}
	// parse list of pods returned
	podNames := strings.Split(etcdPods, "\n")

	// Execute into the working etcd container and setup client TLS authentication in order to be able to communicate
	// with etcd and get output of all etcd members
	prepCmd := fmt.Sprintf("kubectl --kubeconfig <(echo '%s') -n kube-system exec -i %s -- /bin/sh -c ",
		cluster.GetKubeconfig(), podNames[0])

	cmd := fmt.Sprintf("%s \" %s && etcdctl member list \"", prepCmd, exportCmd)
	output, err := exec.Command("bash", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error().Msgf("Error while executing command %s in a working etcd container: %v", cmd, err)
		log.Error().Msgf("prepCmd was %s", prepCmd)
		return err
	}
	// Convert output into []string, each line of output is a separate string
	etcdStrings := strings.Split(string(output), "\n")
	//delete last entry - empty \n
	if len(etcdStrings) > 0 {
		etcdStrings = etcdStrings[:len(etcdStrings)-1]
	}
	// Example etcdNodesOutput:
	// 3ea84f69be8336f3, started, test2-cluster-name1-hetzner-control-2, https://192.168.2.2:2380, https://192.168.2.2:2379, false
	// 56c921bc723229ec, started, test2-cluster-name1-hetzner-control-1, https://192.168.2.1:2380, https://192.168.2.1:2379, false
	var etcdNodeInfos []etcdNodeInfo

	for _, etcdString := range etcdStrings {
		etcdStringTokenized := strings.Split(etcdString, ", ")
		if len(etcdStringTokenized) > 0 {
			temp := etcdNodeInfo{etcdStringTokenized[2] /*name*/, etcdStringTokenized[0] /*hash*/}
			etcdNodeInfos = append(etcdNodeInfos, temp)
		}
	}
	// Remove etcd members that are in etcdToDelete, you need to know an etcd node hash to be able to remove a member
	for _, nodeName := range etcdToDelete {
		for _, etcdNode := range etcdNodeInfos {
			if nodeName == etcdNode.nodeName {
				log.Info().Msgf("Removing node %s, with hash %s \n", etcdNode.nodeName, etcdNode.nodeHash)
				cmd = fmt.Sprintf("%s \" %s && etcdctl member remove %s \"", prepCmd, exportCmd, etcdNode.nodeHash)
				_, err := exec.Command("bash", "-c", cmd).CombinedOutput()
				if err != nil {
					log.Error().Msgf("Error while etcdctl member remove: %v", err)
					log.Error().Msgf("prepCmd was %s", prepCmd)
					return err
				}
			}
		}
	}

	return nil
}

func searchNodeNames(nodeNames []string, nodeNameSubString string) (string, bool) {
	// Get full node name using substring of node name
	for _, nodeName := range nodeNames {
		if strings.Contains(nodeName, nodeNameSubString) {
			return nodeName, true
		}
	}
	return "", false
}

// func getDeleteEtcdCommand() string {

// }

func getEtcdPods(master *pb.Node, cluster *pb.K8Scluster) (string, error) {
	// get etcd pods name
	podsQueryCmd := fmt.Sprintf("kubectl --kubeconfig <(echo \"%s\") %s-%s", cluster.GetKubeconfig(), getEtcdPodsCmd, master.Name)
	output, err := exec.Command("bash", "-c", podsQueryCmd).CombinedOutput()
	if err != nil {
		log.Error().Msgf("Failed to get list of pods with name: etcd-%s", master.Name)
		return "", err
	}
	return string(output), nil
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
