package nodes

import (
	"fmt"
	"strings"

	"github.com/Berops/platform/internal/kubectl"
	"github.com/Berops/platform/internal/utils"
	"github.com/Berops/platform/proto/pb"
	"github.com/rs/zerolog/log"
)

type etcdPodInfo struct {
	nodeName   string
	memberHash string
}

type Deleter struct {
	masterNodes []string
	workerNodes []string
	cluster     *pb.K8Scluster
}

//New returns new Deleter struct, used for node deletion from a k8s cluster
//masterNodes - master nodes to DELETE
//workerNodes - worker nodes to DELETE
func New(masterNodes, workerNodes []string, cluster *pb.K8Scluster) *Deleter {
	return &Deleter{masterNodes: masterNodes, workerNodes: workerNodes, cluster: cluster}
}

//DeleteNodes deletes nodes specified in d.masterNodes and d.workerNodes
//return nil if successful, error otherwise
func (d *Deleter) DeleteNodes() (*pb.K8Scluster, error) {
	kubectl := kubectl.Kubectl{Kubeconfig: d.cluster.Kubeconfig}
	// get real node names
	realNodeNamesBytes, err := kubectl.KubectlGetNodeNames()
	realNodeNames := strings.Split(string(realNodeNamesBytes), "\n")
	if err != nil {
		return nil, fmt.Errorf("error while getting nodes from cluster %s : %v", d.cluster.ClusterInfo.Name, err)
	}
	//delete master nodes + etcd
	err = d.deleteFromEtcd(kubectl)
	if err != nil {
		return nil, fmt.Errorf("error while deleting nodes from etcd for %s : %v", d.cluster.ClusterInfo.Name, err)
	}
	err = deleteNodesByName(kubectl, d.masterNodes, realNodeNames)
	if err != nil {
		return nil, fmt.Errorf("error while deleting nodes from master nodes for %s : %v", d.cluster.ClusterInfo.Name, err)
	}
	//delete worker nodes + nodes.longhorn.io
	err = d.deleteFromLonghorn(kubectl)
	if err != nil {
		return nil, fmt.Errorf("error while deleting nodes.longhorn.io for %s : %v", d.cluster.ClusterInfo.Name, err)
	}
	err = deleteNodesByName(kubectl, d.workerNodes, realNodeNames)
	if err != nil {
		return nil, fmt.Errorf("error while deleting nodes from worker nodes for %s : %v", d.cluster.ClusterInfo.Name, err)
	}
	//update the current cluster
	d.updateClusterData()
	return d.cluster, nil
}

//deleteNodesByName deletes node from cluster by performing
//kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data
//kubectl delete node <node-name>
//return nil if successful, error otherwise
func deleteNodesByName(kc kubectl.Kubectl, nodesToDelete, realNodeNames []string) error {
	for _, nodeName := range nodesToDelete {
		realNodeName := utils.FindName(realNodeNames, nodeName)
		if realNodeName != "" {
			log.Info().Msgf("Deleting node %s from k8s cluster", realNodeName)
			//kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data
			err := kc.KubectlDrain(realNodeName)
			if err != nil {
				return fmt.Errorf("error while draining node %s : %v", nodeName, err)
			}
			//kubectl delete node <node-name>
			err = kc.KubectlDeleteResource("nodes", realNodeName, "")
			if err != nil {
				return fmt.Errorf("error while deleting node %s : %v", nodeName, err)
			}
		} else {
			log.Error().Msgf("Node name that contains %s not found ", nodeName)
			return fmt.Errorf("no node with name %s found ", nodeName)
		}
	}
	return nil
}

//deleteFromEtcd function deletes members of the etcd cluster. This needs to be done in order to prevent any data corruption in etcd
//return nil if successful, error otherwise
func (d *Deleter) deleteFromEtcd(kc kubectl.Kubectl) error {
	mainMasterNode := getMainMaster(d.cluster)
	if mainMasterNode == nil {
		log.Error().Msg("APIEndpoint node not found")
		return fmt.Errorf("failed to find any API endpoint node in cluster %s", d.cluster.ClusterInfo.Name)
	}
	//get etcd pods
	etcdPodsBytes, err := kc.KubectlGetEtcdPods(mainMasterNode.Name)
	etcdPods := strings.Split(string(etcdPodsBytes), "\n")
	if err != nil {
		log.Error().Msgf("Cannot find etcd pods in cluster %s : %v", d.cluster.ClusterInfo.Name, err)
		return fmt.Errorf("cannot find etcd pods in cluster %s  : %v", d.cluster.ClusterInfo.Name, err)
	}

	//get etcd members
	firstEtcdPod := etcdPods[0]
	etcdMembersBytes, err := kc.KubectlExecEtcd(firstEtcdPod, "etcdctl member list")
	if err != nil {
		log.Error().Msgf("Cannot find etcd members in cluster %s : %v", d.cluster.ClusterInfo.Name, err)
		return fmt.Errorf("cannot find etcd members in cluster %s : %v", d.cluster.ClusterInfo.Name, err)
	}
	// Convert output into []string, each line of output is a separate string
	etcdMembersStrings := strings.Split(string(etcdMembersBytes), "\n")
	//delete last entry - empty \n
	if len(etcdMembersStrings) > 0 {
		etcdMembersStrings = etcdMembersStrings[:len(etcdMembersStrings)-1]
	}

	//get pod info, like name of a node where pod is deployed and etcd member hash
	etcdPodInfos := getEtcdPodInfo(etcdMembersStrings)
	// Remove etcd members that are in mastersToDelete, you need to know an etcd node hash to be able to remove a member
	for _, nodeName := range d.masterNodes {
		for _, etcdPodInfo := range etcdPodInfos {
			if nodeName == etcdPodInfo.nodeName {
				log.Info().Msgf("Deleting node %s, with etcd member hash %s ", etcdPodInfo.nodeName, etcdPodInfo.memberHash)
				etcdctlCmd := fmt.Sprintf("etcdctl member remove %s", etcdPodInfo.memberHash)
				_, err := kc.KubectlExecEtcd(firstEtcdPod, etcdctlCmd)
				if err != nil {
					log.Error().Msgf("Error while etcdctl member remove: %v", err)
					return err
				}
			}
		}
	}
	return nil
}

//deleteFromLonghorn will delete node from nodes.longhorn.io
//return nil if successful, error otherwise
func (d *Deleter) deleteFromLonghorn(kc kubectl.Kubectl) error {
	for _, nodeName := range d.workerNodes {
		log.Info().Msgf("Deleting node %s from nodes.longhorn.io", nodeName)
		err := kc.KubectlDeleteResource("nodes.longhorn.io", nodeName, "longhorn-system")
		if err != nil {
			return err
		}
	}
	return nil
}

//getMainMaster iterates over all control nodes in cluster and returns API EP node
//return API EP node if successful, nil otherwise
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

//getEtcdPodInfo tokenizes an etcdMemberInfo and data containing node name and etcd member hash for all etcd members
//return slice of etcdPodInfo containing node name and etcd member hash for all etcd members
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

//updateClusterData will remove deleted nodes from nodepools
func (d *Deleter) updateClusterData() {
	for _, name := range append(d.masterNodes, d.workerNodes...) {
		for _, nodepool := range d.cluster.ClusterInfo.NodePools {
			for i, node := range nodepool.Nodes {
				if node.Name == name {
					nodepool.Count--
					nodepool.Nodes = append(nodepool.Nodes[:i], nodepool.Nodes[i+1:]...)
				}
			}
		}
	}
}
