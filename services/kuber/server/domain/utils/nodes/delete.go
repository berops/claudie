package nodes

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/berops/claudie/internal/clusters"
	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
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

type nodeInfo struct {
	fullname       string
	k8sName        string
	publicEndpoint string
}

type Deleter struct {
	masterNodes   []nodeInfo
	workerNodes   []nodeInfo
	cluster       *spec.K8Scluster
	clusterPrefix string

	logger zerolog.Logger
}

// New returns new Deleter struct, used for node deletion from a k8s cluster
// masterNodes - master nodes to DELETE
// workerNodes - worker nodes to DELETE
func NewDeleter(masterNodes, workerNodes []string, cluster *spec.K8Scluster) *Deleter {
	clusterID := cluster.ClusterInfo.Id()
	var mn, wn []nodeInfo

	for i := range masterNodes {
		mn = append(mn, nodeInfo{
			fullname:       masterNodes[i],
			k8sName:        strings.TrimPrefix(masterNodes[i], fmt.Sprintf("%s-", clusterID)),
			publicEndpoint: clusters.NodePublic(masterNodes[i], cluster),
		})
	}

	for i := range workerNodes {
		wn = append(wn, nodeInfo{
			fullname:       workerNodes[i],
			k8sName:        strings.TrimPrefix(workerNodes[i], fmt.Sprintf("%s-", clusterID)),
			publicEndpoint: clusters.NodePublic(workerNodes[i], cluster),
		})
	}

	return &Deleter{
		masterNodes:   mn,
		workerNodes:   wn,
		cluster:       cluster,
		clusterPrefix: clusterID,

		logger: loggerutils.WithClusterName(clusterID),
	}
}

// DeleteNodes deletes nodes specified in d.masterNodes and d.workerNodes
// return nil if successful, error otherwise
func (d *Deleter) DeleteNodes() (*spec.K8Scluster, error) {
	kubectl := kubectl.Kubectl{Kubeconfig: d.cluster.Kubeconfig, MaxKubectlRetries: 5}
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

	// Remove worker nodes sequentially to minimise risk of fault when replicating PVC
	var errDel error
	for _, worker := range d.workerNodes {
		if !slices.Contains(realNodeNames, worker.k8sName) {
			d.logger.Warn().Msgf("Node with name %s not found in cluster", worker.k8sName)
			continue
		}

		if err := kubectl.KubectlCordon(worker.k8sName); err != nil {
			errDel = errors.Join(errDel, fmt.Errorf("error while cordon worker node %s from cluster %s: %w", worker.k8sName, d.clusterPrefix, err))
			continue
		}

		if err := d.deleteNodesByName(kubectl, worker, realNodeNames); err != nil {
			errDel = errors.Join(errDel, fmt.Errorf("error while deleting node %s from cluster %s: %w", worker.k8sName, d.clusterPrefix, err))
			continue
		}

		if err := removeReplicasOnDeletedNode(kubectl, worker.k8sName); err != nil {
			// not a fatal error.
			d.logger.Warn().Msgf("failed to delete unused replicas from replicas.longhorn.io, after node %s deletion: %s", worker.k8sName, err)
		}
	}
	if errDel != nil {
		return nil, errDel
	}

	// Update the current cluster
	d.updateClusterData()
	return d.cluster, nil
}

// deleteNodesByName deletes node from the k8s cluster.
func (d *Deleter) deleteNodesByName(kc kubectl.Kubectl, node nodeInfo, realNodeNames []string) error {
	if !slices.Contains(realNodeNames, node.k8sName) {
		d.logger.Warn().Msgf("Node with name %s not found in cluster", node.k8sName)
		return nil
	}

	d.logger.Info().Msgf("verifying if node %s is reachable", node.k8sName)
	if err := clusters.Ping(d.logger, clusters.PingRetryCount, node.publicEndpoint); err != nil {
		if errors.Is(err, clusters.ErrEchoTimeout) {
			d.logger.Info().Msgf("Node %s is unreachable, marking node with `out-of-service` taint before deleting it from the cluster", node.k8sName)
			if err := kc.KubectlTaintNodeShutdown(node.k8sName); err != nil {
				d.logger.Err(err).Msgf("Failed to taint node %s with 'out-of-service' taint, proceeding with default deletion", node.k8sName)
			}
		} else {
			d.logger.Err(err).Msgf("Failed to determine if node %s is reachable or not, proceeding with default deletion", node.k8sName)
		}
	}

	d.logger.Info().Msgf("Deleting node %s from k8s cluster", node.k8sName)

	//kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data
	if err := kc.KubectlDrain(node.k8sName); err != nil {
		return fmt.Errorf("error while draining node %s from cluster %s : %w", node.k8sName, d.clusterPrefix, err)
	}

	//kubectl delete node <node-name>
	if err := kc.KubectlDeleteResource("nodes", node.k8sName); err != nil {
		return fmt.Errorf("error while deleting node %s from cluster %s : %w", node.k8sName, d.clusterPrefix, err)
	}

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
	for _, node := range d.masterNodes {
		for _, etcdPodInfo := range etcdPodInfos {
			if node.k8sName == etcdPodInfo.nodeName {
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
	for _, deleted := range append(d.masterNodes, d.workerNodes...) {
		nodepools.DeleteNodeByName(d.cluster.ClusterInfo.NodePools, deleted.fullname)
	}
}

// getMainMaster iterates over all control nodes in cluster and returns API EP node. If none defined with this type,
// function returns any master node which will not be deleted.
// return API EP node if successful, nil otherwise
func (d *Deleter) getMainMaster() *spec.Node {
	if _, n := nodepools.FindApiEndpoint(d.cluster.ClusterInfo.NodePools); n != nil {
		return n
	}

	// Choose one master, which is not going to be deleted
	for np := range nodepools.Control(d.cluster.ClusterInfo.GetNodePools()) {
		for _, n := range np.Nodes {
			contains := slices.ContainsFunc(d.masterNodes, func(mn nodeInfo) bool {
				return mn.fullname == n.Name
			})
			if !contains {
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
