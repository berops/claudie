package nodes

import (
	"fmt"
	"strings"
	"time"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	longhornNamespace     = "longhorn-system"
	pvcReplicationTimeout = 10 * time.Second
)

type etcdPodInfo struct {
	nodeName   string
	memberHash string
}

type Deleter struct {
	masterNodes   []string
	workerNodes   []string
	cluster       *pb.K8Scluster
	clusterPrefix string
}

// New returns new Deleter struct, used for node deletion from a k8s cluster
// masterNodes - master nodes to DELETE
// workerNodes - worker nodes to DELETE
func New(masterNodes, workerNodes []string, cluster *pb.K8Scluster) *Deleter {
	prefix := fmt.Sprintf("%s-%s-", cluster.ClusterInfo.Name, cluster.ClusterInfo.Hash)

	for i := range masterNodes {
		masterNodes[i] = strings.TrimPrefix(masterNodes[i], prefix)
	}

	for i := range workerNodes {
		workerNodes[i] = strings.TrimPrefix(workerNodes[i], prefix)
	}

	return &Deleter{
		masterNodes:   masterNodes,
		workerNodes:   workerNodes,
		cluster:       cluster,
		clusterPrefix: prefix,
	}
}

// DeleteNodes deletes nodes specified in d.masterNodes and d.workerNodes
// return nil if successful, error otherwise
func (d *Deleter) DeleteNodes() (*pb.K8Scluster, error) {
	kubectl := kubectl.Kubectl{Kubeconfig: d.cluster.Kubeconfig}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", d.cluster.ClusterInfo.Name, d.cluster.ClusterInfo.Hash)
		kubectl.Stdout = comm.GetStdOut(prefix)
		kubectl.Stderr = comm.GetStdErr(prefix)
	}
	// get real node names
	realNodeNamesBytes, err := kubectl.KubectlGetNodeNames()
	realNodeNames := strings.Split(string(realNodeNamesBytes), "\n")
	if err != nil {
		return nil, fmt.Errorf("error while getting nodes from cluster %s : %w", d.cluster.ClusterInfo.Name, err)
	}

	etcdEpNode := d.getMainMaster()
	for _, master := range d.masterNodes {
		// delete master nodes from etcd
		if err := d.deleteFromEtcd(kubectl, etcdEpNode); err != nil {
			return nil, fmt.Errorf("error while deleting nodes from etcd for %s : %w", d.cluster.ClusterInfo.Name, err)
		}
		// delete master nodes
		if err := deleteNodesByName(kubectl, master, realNodeNames); err != nil {
			return nil, fmt.Errorf("error while deleting nodes from master nodes for %s : %w", d.cluster.ClusterInfo.Name, err)
		}
	}

	for _, worker := range d.workerNodes {
		// assure replication of storage
		if err := assureReplication(kubectl, worker); err != nil {
			return nil, fmt.Errorf("error while making sure storage is replicated before deletion on cluster %s : %w", d.cluster.ClusterInfo.Name, err)
		}
		// delete worker nodes from nodes.longhorn.io
		if err := deleteFromLonghorn(kubectl, worker); err != nil {
			return nil, fmt.Errorf("error while deleting nodes.longhorn.io for %s : %w", d.cluster.ClusterInfo.Name, err)
		}
		// delete worker nodes
		if err := deleteNodesByName(kubectl, worker, realNodeNames); err != nil {
			return nil, fmt.Errorf("error while deleting nodes from worker nodes for %s : %w", d.cluster.ClusterInfo.Name, err)
		}
	}

	// update the current cluster
	d.updateClusterData()
	return d.cluster, nil
}

// deleteNodesByName deletes node from cluster by performing
// kubectl delete node <node-name>
// return nil if successful, error otherwise
func deleteNodesByName(kc kubectl.Kubectl, nodeName string, realNodeNames []string) error {
	realNodeName := utils.FindName(realNodeNames, nodeName)
	if realNodeName != "" {
		log.Debug().Msgf("Deleting node %s from k8s cluster", realNodeName)
		//kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data
		err := kc.KubectlDrain(realNodeName)
		if err != nil {
			return fmt.Errorf("error while draining node %s : %w", nodeName, err)
		}
		//kubectl delete node <node-name>
		err = kc.KubectlDeleteResource("nodes", realNodeName, "")
		if err != nil {
			return fmt.Errorf("error while deleting node %s : %w", nodeName, err)
		}
	} else {
		log.Error().Msgf("Node name that contains %s not found ", nodeName)
		return fmt.Errorf("no node with name %s found ", nodeName)
	}
	return nil
}

// deleteFromEtcd function deletes members of the etcd cluster. This needs to be done in order to prevent any data corruption in etcd
// return nil if successful, error otherwise
func (d *Deleter) deleteFromEtcd(kc kubectl.Kubectl, etcdEpNode *pb.Node) error {
	//get etcd pods
	etcdPods, err := getEtcdPodNames(kc, strings.TrimPrefix(etcdEpNode.Name, d.clusterPrefix))
	if err != nil {
		return fmt.Errorf("cannot find etcd pods in cluster %s  : %w", d.cluster.ClusterInfo.Name, err)
	}
	etcdMembers, err := getEtcdMembers(kc, etcdPods[0])
	if err != nil {
		return fmt.Errorf("cannot find etcd members in cluster %s : %w", d.cluster.ClusterInfo.Name, err)
	}
	//get pod info, like name of a node where pod is deployed and etcd member hash
	etcdPodInfos := getEtcdPodInfo(etcdMembers)
	// Remove etcd members that are in mastersToDelete, you need to know an etcd node hash to be able to remove a member
	for _, nodeName := range d.masterNodes {
		for _, etcdPodInfo := range etcdPodInfos {
			if nodeName == etcdPodInfo.nodeName {
				log.Debug().Msgf("Deleting etcd member %s, with hash %s ", etcdPodInfo.nodeName, etcdPodInfo.memberHash)
				etcdctlCmd := fmt.Sprintf("etcdctl member remove %s", etcdPodInfo.memberHash)
				_, err := kc.KubectlExecEtcd(etcdPods[0], etcdctlCmd)
				if err != nil {
					return fmt.Errorf("error while executing \"etcdctl member remove\" on node %s, cluster %s: %w", etcdPodInfo.nodeName, d.cluster.ClusterInfo.Name, err)
				}
			}
		}
	}
	return nil
}

// updateClusterData will remove deleted nodes from nodepools
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

// deleteFromLonghorn will delete node from nodes.longhorn.io
// return nil if successful, error otherwise
func deleteFromLonghorn(kc kubectl.Kubectl, worker string) error {
	log.Info().Msgf("Deleting node %s from nodes.longhorn.io", worker)
	if err := kc.KubectlDeleteResource("nodes.longhorn.io", worker, "-n", longhornNamespace); err != nil {
		return fmt.Errorf("error while deleting node %s from nodes.longhorn.io : %w", worker, err)
	}
	return nil
}

// assureReplication tries to assure, that replicas for each longhorn volume are migrated to nodes, which will remain in the cluster.
func assureReplication(kc kubectl.Kubectl, worker string) error {
	// Get replicas and volumes as they can be scheduled on next node, which will be deleted.
	replicas, err := getReplicas(kc)
	if err != nil {
		return fmt.Errorf("error while getting replicas from cluster : %w", err)
	}
	volumes, err := getVolumes(kc)
	if err != nil {
		return fmt.Errorf("error while getting volumes from cluster  : %w", err)
	}
	if reps, ok := replicas[worker]; ok {
		for _, r := range reps {
			// Try to force creation of a new replicas from node, which will be deleted.
			if v, ok := volumes[r.Spec.VolumeName]; ok {
				// Increase number of replicas in volume.
				if err := increaseReplicaCount(v, kc); err != nil {
					return fmt.Errorf("error while increasing number of replicas in volume %s from cluster : %w", v.Metadata.Name, err)
				}
				// Wait pvcReplicationTimeout for Longhorn to create new replica.
				log.Info().Msgf("Waiting %.0f seconds for new replicas to be scheduled if possible for node %s", pvcReplicationTimeout.Seconds(), worker)
				time.Sleep(pvcReplicationTimeout)
				// Decrease number of replicas in volume -> original state.
				if err := revertReplicaCount(v, kc); err != nil {
					return fmt.Errorf("error while increasing number of replicas in volume %s : %w", v.Metadata.Name, err)
				}
				// Delete old replica, on to-be-deleted node.
				log.Debug().Msgf("Deleting replica %s from node %s", r.Metadata.Name, r.Status.OwnerID)
				if err := deleteReplica(r, kc); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// getMainMaster iterates over all control nodes in cluster and returns API EP node. If none defined with this type,
// function returns any master node which will not be deleted.
// return API EP node if successful, nil otherwise
func (d *Deleter) getMainMaster() *pb.Node {
	for _, nodepool := range d.cluster.ClusterInfo.GetNodePools() {
		for _, node := range nodepool.Nodes {
			if node.NodeType == pb.NodeType_apiEndpoint {
				return node
			}
		}
	}
	// Choose one master, which is not going to be deleted
	for _, nodepool := range d.cluster.ClusterInfo.GetNodePools() {
	node:
		for _, node := range nodepool.Nodes {
			if node.NodeType == pb.NodeType_master {
				// If node will be deleted, continue.
				for _, dm := range d.masterNodes {
					if strings.Contains(node.Name, dm) {
						continue node
					}
				}
			}
			// If loop was not broken by the continue, return this node.
			return node
		}
	}
	return nil
}

// getEtcdPodNames returns slice of strings containing all etcd pod names
func getEtcdPodNames(kc kubectl.Kubectl, masterNodeName string) ([]string, error) {
	etcdPodsBytes, err := kc.KubectlGetEtcdPods(masterNodeName)
	if err != nil {
		return nil, fmt.Errorf("cannot find etcd pods in cluster with master node %s : %w", masterNodeName, err)
	}
	return strings.Split(string(etcdPodsBytes), "\n"), nil
}

// getEtcdMembers will return slice of strings, each element containing etcd member info from "etcdctl member list"
//
// Example output:
// [
// "3ea84f69be8336f3, started, test2-cluster-name1-hetzner-control-2, https://192.168.2.2:2380, https://192.168.2.2:2379, false",
// "56c921bc723229ec, started, test2-cluster-name1-hetzner-control-1, https://192.168.2.1:2380, https://192.168.2.1:2379, false"
// ]
func getEtcdMembers(kc kubectl.Kubectl, etcdPod string) ([]string, error) {
	//get etcd members
	etcdMembersBytes, err := kc.KubectlExecEtcd(etcdPod, "etcdctl member list")
	if err != nil {
		return nil, fmt.Errorf("cannot find etcd members in cluster with etcd pod %s : %w", etcdPod, err)
	}
	// Convert output into []string, each line of output is a separate string
	etcdMembersStrings := strings.Split(string(etcdMembersBytes), "\n")
	//delete last entry - empty \n
	if len(etcdMembersStrings) > 0 {
		etcdMembersStrings = etcdMembersStrings[:len(etcdMembersStrings)-1]
	}
	return etcdMembersStrings, nil
}

// getEtcdPodInfo tokenizes an etcdMemberInfo and data containing node name and etcd member hash for all etcd members
// return slice of etcdPodInfo containing node name and etcd member hash for all etcd members
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
