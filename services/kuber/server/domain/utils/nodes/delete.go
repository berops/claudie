package nodes

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
)

const (
	longhornNamespace = "longhorn-system"
)

type etcdPodInfo struct {
	nodeName   string
	memberHash string
}

type Deleter struct {
	masterNodes   []string
	workerNodes   []string
	cluster       *spec.K8Scluster
	clusterPrefix string

	logger zerolog.Logger
}

// New returns new Deleter struct, used for node deletion from a k8s cluster
// masterNodes - master nodes to DELETE
// workerNodes - worker nodes to DELETE
func NewDeleter(masterNodes, workerNodes []string, cluster *spec.K8Scluster) *Deleter {
	prefix := utils.GetClusterID(cluster.ClusterInfo)

	for i := range masterNodes {
		masterNodes[i] = strings.TrimPrefix(masterNodes[i], fmt.Sprintf("%s-", prefix))
	}

	for i := range workerNodes {
		workerNodes[i] = strings.TrimPrefix(workerNodes[i], fmt.Sprintf("%s-", prefix))
	}

	return &Deleter{
		masterNodes:   masterNodes,
		workerNodes:   workerNodes,
		cluster:       cluster,
		clusterPrefix: prefix,

		logger: utils.CreateLoggerWithClusterName(prefix),
	}
}

// DeleteNodes deletes nodes specified in d.masterNodes and d.workerNodes
// return nil if successful, error otherwise
func (d *Deleter) DeleteNodes() (*spec.K8Scluster, error) {
	kubectl := kubectl.Kubectl{Kubeconfig: d.cluster.Kubeconfig, MaxKubectlRetries: 3}
	kubectl.Stdout = comm.GetStdOut(d.clusterPrefix)
	kubectl.Stderr = comm.GetStdErr(d.clusterPrefix)

	// get real node names
	realNodeNamesBytes, err := kubectl.KubectlGetNodeNames()
	realNodeNames := strings.Split(string(realNodeNamesBytes), "\n")
	if err != nil {
		return nil, fmt.Errorf("error while getting nodes from cluster %s : %w", d.clusterPrefix, err)
	}

	etcdEpNode := d.getMainMaster()
	// Remove master nodes sequentially to minimise risk of faults in etcd
	for _, master := range d.masterNodes {
		// delete master nodes from etcd
		if err := d.deleteFromEtcd(kubectl, etcdEpNode); err != nil {
			return nil, fmt.Errorf("error while deleting nodes from etcd for %s : %w", d.clusterPrefix, err)
		}
		// delete master nodes
		if err := d.deleteNodesByName(kubectl, master, realNodeNames); err != nil {
			return nil, fmt.Errorf("error while deleting nodes from master nodes for %s : %w", d.clusterPrefix, err)
		}
	}

	// Cordon worker nodes to prevent any new pods/volume replicas being scheduled there
	if err := utils.ConcurrentExec(d.workerNodes, func(_ int, worker string) error {
		if realNodeName := utils.FindName(realNodeNames, worker); realNodeName != "" {
			return kubectl.KubectlCordon(worker)
		}
		d.logger.Warn().Msgf("Node name %s not found in cluster.", worker)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("error while cordoning worker nodes from cluster %s which were marked for deletion : %w", d.clusterPrefix, err)
	}

	// Remove worker nodes sequentially to minimise risk of fault when replicating PVC
	var errDel error
	for _, worker := range d.workerNodes {
		// Delete worker nodes
		if err := d.deleteNodesByName(kubectl, worker, realNodeNames); err != nil {
			errDel = errors.Join(errDel, fmt.Errorf("error while deleting worker node %s from cluster %s: %w", worker, d.clusterPrefix, err))
			continue
		}

		if d.isNodeDynamic(worker) {
			// we delete the failed replicas on the dynamic nodes as they same volumes will definitely
			// not be reused again. This might not be the case for static nodes where the same volume
			// may be reused.
			if err := deleteReplicaOnNode(kubectl, worker); err != nil {
				// not a fatal error.
				d.logger.Warn().Msgf("failed to delete unused replica from replicas.longhorn.io, after node %s deletion: %s", worker, err)
			}
		}
	}
	if errDel != nil {
		return nil, errDel
	}

	// Update the current cluster
	d.updateClusterData()
	return d.cluster, nil
}

func (d *Deleter) isNodeDynamic(worker string) bool {
	for _, n := range utils.GetCommonDynamicNodePools(d.cluster.ClusterInfo.NodePools) {
		if n.GetStaticNodePool() != nil {
			continue
		}

		for _, n := range n.Nodes {
			if n.Name == worker {
				return true
			}
		}
	}
	return false
}

// deleteNodesByName deletes node from the k8s cluster.
func (d *Deleter) deleteNodesByName(kc kubectl.Kubectl, nodeName string, realNodeNames []string) error {
	if realNodeName := utils.FindName(realNodeNames, nodeName); realNodeName != "" {
		d.logger.Info().Msgf("Deleting node %s from k8s cluster", realNodeName)

		if err := kc.KubectlDrain(realNodeName); err != nil {
			return fmt.Errorf("error while draining node %s from cluster %s : %w", nodeName, d.clusterPrefix, err)
		}

		if err := kc.KubectlDeleteResource("nodes", realNodeName); err != nil {
			return fmt.Errorf("error while deleting node %s from cluster %s : %w", nodeName, d.clusterPrefix, err)
		}

		return nil
	}

	d.logger.Warn().Msgf("Node name that contains %s not found in cluster", nodeName)
	return nil
}

// deleteFromEtcd function deletes members of the etcd cluster. This needs to be done in order to prevent any data corruption in etcd
// return nil if successful, error otherwise
func (d *Deleter) deleteFromEtcd(kc kubectl.Kubectl, etcdEpNode *spec.Node) error {
	//get etcd pods
	etcdPods, err := getEtcdPodNames(kc, strings.TrimPrefix(etcdEpNode.Name, fmt.Sprintf("%s-", d.clusterPrefix)))
	if err != nil {
		return fmt.Errorf("cannot find etcd pods in cluster %s  : %w", d.clusterPrefix, err)
	}
	etcdMembers, err := getEtcdMembers(kc, etcdPods[0])
	if err != nil {
		return fmt.Errorf("cannot find etcd members in cluster %s : %w", d.clusterPrefix, err)
	}
	//get pod info, like name of a node where pod is deployed and etcd member hash
	etcdPodInfos := getEtcdPodInfo(etcdMembers)
	// Remove etcd members that are in mastersToDelete, you need to know an etcd node hash to be able to remove a member
	for _, nodeName := range d.masterNodes {
		for _, etcdPodInfo := range etcdPodInfos {
			if nodeName == etcdPodInfo.nodeName {
				d.logger.Debug().Msgf("Deleting etcd member %s, with hash %s", etcdPodInfo.nodeName, etcdPodInfo.memberHash)
				etcdctlCmd := fmt.Sprintf("etcdctl member remove %s", etcdPodInfo.memberHash)
				_, err := kc.KubectlExecEtcd(etcdPods[0], etcdctlCmd)
				if err != nil {
					return fmt.Errorf("error while executing \"etcdctl member remove\" on node %s, cluster %s: %w", etcdPodInfo.nodeName, d.clusterPrefix, err)
				}
			}
		}
	}
	return nil
}

// updateClusterData will remove deleted nodes from nodepools
func (d *Deleter) updateClusterData() {
nodes:
	for _, name := range append(d.masterNodes, d.workerNodes...) {
		for _, np := range d.cluster.ClusterInfo.NodePools {
			for i, node := range np.Nodes {
				if node.Name == name {
					np.Nodes = append(np.Nodes[:i], np.Nodes[i+1:]...)
					continue nodes
				}
			}
		}
	}
}

// getMainMaster iterates over all control nodes in cluster and returns API EP node. If none defined with this type,
// function returns any master node which will not be deleted.
// return API EP node if successful, nil otherwise
func (d *Deleter) getMainMaster() *spec.Node {
	n, err := utils.FindAPIEndpointNode(d.cluster.ClusterInfo.NodePools)
	if err == nil {
		return n
	}

	// Choose one master, which is not going to be deleted
	for _, np := range d.cluster.ClusterInfo.GetNodePools() {
		if !np.IsControl {
			continue
		}

		for _, n := range np.Nodes {
			name := strings.TrimPrefix(n.Name, fmt.Sprintf("%s-", d.clusterPrefix))
			if !slices.Contains(d.masterNodes, name) {
				return n
			}
		}
	}

	panic("no master nodes or api endpoint node, malformed state, can't continue")
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
