package nodes

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

const (
	longhornNamespace         = "longhorn-system"
	newReplicaCreationTimeout = 10 * time.Second
)

type (
	etcdPodInfo struct {
		nodeName   string
		memberHash string
	}

	NodeDeleter struct {
		cluster   *pb.K8Scluster
		clusterID string

		// Nodes to be deleted.
		masterNodes []string
		workerNodes []string

		logger zerolog.Logger
	}
)

// NewNodeDeleter returns an instance of NodeDeleter, used for deleting nodes from a K8s cluster.
func NewNodeDeleter(masterNodes, workerNodes []string, cluster *pb.K8Scluster) *NodeDeleter {
	clusterID := fmt.Sprintf("%s-%s", cluster.ClusterInfo.Name, cluster.ClusterInfo.Hash)

	for i := range masterNodes {
		masterNodes[i] = strings.TrimPrefix(masterNodes[i], fmt.Sprintf("%s-", clusterID))
	}
	for i := range workerNodes {
		workerNodes[i] = strings.TrimPrefix(workerNodes[i], fmt.Sprintf("%s-", clusterID))
	}

	return &NodeDeleter{
		cluster:   cluster,
		clusterID: clusterID,

		masterNodes: masterNodes,
		workerNodes: workerNodes,

		logger: utils.CreateLoggerWithClusterName(clusterID),
	}
}

// DeleteNodes deletes nodes specified in d.masterNodes and d.workerNodes.
// Returns nil if successful, error otherwise.
func (d *NodeDeleter) DeleteNodes() (*pb.K8Scluster, error) {
	kubectl := kubectl.Kubectl{Kubeconfig: d.cluster.Kubeconfig, MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		kubectl.Stdout = comm.GetStdOut(d.clusterID)
		kubectl.Stderr = comm.GetStdErr(d.clusterID)
	}

	// Get node names
	nodeNamesBytes, err := kubectl.KubectlGetNodeNames()
	if err != nil {
		return nil, fmt.Errorf("error while getting nodes from cluster %s : %w", d.clusterID, err)
	}
	nodeNames := strings.Split(string(nodeNamesBytes), "\n")

	apiEndpointNode := d.getApiEndpointNode()

	// Remove master nodes sequentially to minimise risk of faults in ETCD.
	for _, masterNode := range d.masterNodes {
		// Delete ETCD nodes running in given master nodes.
		if err := d.deleteETCDNodesFromETCDCluster(kubectl, apiEndpointNode); err != nil {
			return nil, fmt.Errorf("error while deleting nodes from etcd for %s : %w", d.clusterID, err)
		}

		// Then delete master nodes.
		if err := d.deleteNodesByName(kubectl, masterNode, nodeNames); err != nil {
			return nil, fmt.Errorf("error while deleting nodes from master nodes for %s : %w", d.clusterID, err)
		}
	}

	// Cordon worker nodes to prevent any new pods/volume replicas from being scheduled there.
	if err := utils.ConcurrentExec(d.workerNodes,
		func(worker string) error {
			return kubectl.KubectlCordon(worker)
		},
	); err != nil {
		return nil, fmt.Errorf("error while cordoning worker nodes from cluster %s which were marked for deletion : %w", d.clusterID, err)
	}

	// Remove worker nodes sequentially to minimise risk of fault when replicating PVC.
	for _, workerNode := range d.workerNodes {
		// Assure replication of storage
		if err := d.assureReplication(kubectl, workerNode); err != nil {
			return nil, fmt.Errorf("error while making sure storage is replicated before deletion on cluster %s : %w", d.clusterID, err)
		}
		// Delete worker nodes from nodes.longhorn.io
		if err := d.deregisterNodeFromLonghorn(kubectl, workerNode); err != nil {
			return nil, fmt.Errorf("error while deleting nodes.longhorn.io for %s : %w", d.clusterID, err)
		}

		// Delete worker nodes
		if err := d.deleteNodesByName(kubectl, workerNode, nodeNames); err != nil {
			return nil, fmt.Errorf("error while deleting nodes from worker nodes for %s : %w", d.clusterID, err)
		}
		// NOTE: Might need to manually verify if the volume got detached.
		// https://github.com/berops/claudie/issues/784
	}

	// Update the current cluster
	d.updateClusterData()

	return d.cluster, nil
}

// getApiEndpointNode iterates over all control nodes in the K8s cluster and returns API Endpoint node.
// If none defined with this type, function returns any master node which will not be deleted.
// Returns API Endpoint node if found, nil otherwise.
func (d *NodeDeleter) getApiEndpointNode() *pb.Node {
	for _, nodepool := range d.cluster.ClusterInfo.GetNodePools() {
		for _, node := range nodepool.Nodes {
			if node.NodeType == pb.NodeType_apiEndpoint {
				return node
			}
		}
	}

	// Choose one master node, which is not going to be deleted.
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
			// If loop was not broken by the continue, return this master node.
			return node
		}
	}

	return nil
}

// deleteETCDNodesFromETCDCluster function deletes members of the ETCD cluster. This needs to be
// done in order to prevent any data corruption in ETCD.
// Returns nil if successful, error otherwise
func (d *NodeDeleter) deleteETCDNodesFromETCDCluster(kc kubectl.Kubectl, etcdEpNode *pb.Node) error {
	etcdPods, err := getEtcdPodNames(kc, strings.TrimPrefix(etcdEpNode.Name, fmt.Sprintf("%s-", d.clusterID)))
	if err != nil {
		return fmt.Errorf("cannot find etcd pods in cluster %s  : %w", d.clusterID, err)
	}
	etcdMembers, err := getEtcdMembers(kc, etcdPods[0])
	if err != nil {
		return fmt.Errorf("cannot find etcd members in cluster %s : %w", d.clusterID, err)
	}

	// get ETCD pod info (like name of a node where pod is deployed and ETCD member hash)
	etcdPodInfos := getEtcdPodInfo(etcdMembers)

	// Remove ETCD members that are in d.masterNodes.
	// You need to know ETCD node hash to be able to remove the member.
	for _, nodeName := range d.masterNodes {
		for _, etcdPodInfo := range etcdPodInfos {
			if nodeName == etcdPodInfo.nodeName {
				d.logger.Debug().Msgf("Deleting etcd member %s, with hash %s", etcdPodInfo.nodeName, etcdPodInfo.memberHash)

				etcdctlCmd := fmt.Sprintf("etcdctl member remove %s", etcdPodInfo.memberHash)
				if _, err := kc.KubectlExecEtcd(etcdPods[0], etcdctlCmd); err != nil {
					return fmt.Errorf("error while executing \"etcdctl member remove\" on node %s, cluster %s: %w", etcdPodInfo.nodeName, d.clusterID, err)
				}
			}
		}
	}

	return nil
}

// deleteNodesByName deletes node from cluster by performing
// "kubectl delete node <node-name>".
// Returns nil if successful, error otherwise.
func (d *NodeDeleter) deleteNodesByName(kc kubectl.Kubectl, nodeName string, nodeNames []string) error {
	// TODO: understand when this if statement will not pass.
	if nodeName := utils.FindName(nodeNames, nodeName); nodeName != "" {
		d.logger.Info().Msgf("Deleting node %s from k8s cluster", nodeName)

		// kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data
		err := kc.KubectlDrain(nodeName)
		if err != nil {
			return fmt.Errorf("error while draining node %s from cluster %s : %w", nodeName, d.clusterID, err)
		}
		// kubectl delete node <node-name>
		err = kc.KubectlDeleteResource("nodes", nodeName)
		if err != nil {
			return fmt.Errorf("error while deleting node %s from cluster %s : %w", nodeName, d.clusterID, err)
		}

		return nil
	}

	d.logger.Warn().Msgf("Node name that contains %s not found in cluster", nodeName)
	return nil
}

// getEtcdPodNames returns names of pods running ETCD.
func getEtcdPodNames(kc kubectl.Kubectl, masterNodeName string) ([]string, error) {
	etcdPodsBytes, err := kc.KubectlGetEtcdPods(masterNodeName)
	if err != nil {
		return nil, fmt.Errorf("cannot find etcd pods in cluster with master node %s : %w", masterNodeName, err)
	}
	return strings.Split(string(etcdPodsBytes), "\n"), nil
}

// getEtcdMembers will return slice of strings,
// each element containing ETCD member info outputted by "etcdctl member list".
//
// Example output:
// [
//
//	"3ea84f69be8336f3, started, test2-cluster-name1-hetzner-control-2, https://192.168.2.2:2380, https://192.168.2.2:2379, false",
//	"56c921bc723229ec, started, test2-cluster-name1-hetzner-control-1, https://192.168.2.1:2380, https://192.168.2.1:2379, false"
//
// ]
func getEtcdMembers(kc kubectl.Kubectl, etcdPodName string) ([]string, error) {
	// Get ETCD members
	etcdMembersBytes, err := kc.KubectlExecEtcd(etcdPodName, "etcdctl member list")
	if err != nil {
		return nil, fmt.Errorf("cannot find ETCD members in cluster ETCD etcd pod %s : %w", etcdPodName, err)
	}

	// Convert output into []string, each line of output is a separate string
	etcdMembersInfoStrings := strings.Split(string(etcdMembersBytes), "\n")
	// Delete last entry - empty \n
	if len(etcdMembersInfoStrings) > 0 {
		etcdMembersInfoStrings = etcdMembersInfoStrings[:len(etcdMembersInfoStrings)-1]
	}

	return etcdMembersInfoStrings, nil
}

// getEtcdPodInfo tokenizes an etcdMemberInfoString and data containing node name and ETCD member hash
// for all ETCD members.
// Returns slice of etcdPodInfo containing node name and ETCD member hash for all ETCD members.
func getEtcdPodInfo(etcdMembersInfoStrings []string) []etcdPodInfo {
	var etcdPodInfos []etcdPodInfo

	for _, etcdMemberInfoString := range etcdMembersInfoStrings {
		etcdMemberInfoStringTokenized := strings.Split(etcdMemberInfoString, ", ")
		if len(etcdMemberInfoStringTokenized) > 0 {
			etcdPodInfos = append(etcdPodInfos,
				etcdPodInfo{
					etcdMemberInfoStringTokenized[2], /*name*/
					etcdMemberInfoStringTokenized[0], /*hash*/
				},
			)
		}
	}

	return etcdPodInfos
}

// deregisterNodeFromLonghorn will delete node from nodes.longhorn.io.
// Returns nil if successful, error otherwise.
func (d *NodeDeleter) deregisterNodeFromLonghorn(kc kubectl.Kubectl, worker string) error {
	// Check if the resource is present before deleting.
	if logs, err := kc.KubectlGet(fmt.Sprintf("nodes.longhorn.io %s", worker), "-n", longhornNamespace); err != nil {
		// This is not the ideal path of checking for a NotFound error,
		// this is only done as we shell out to run kubectl.
		if strings.Contains(string(logs), "NotFound") {
			d.logger.Warn().Msgf("worker node: %s not found, assuming it was deleted.", worker)
			return nil
		}
	}

	d.logger.Info().Msgf("Deleting node %s from nodes.longhorn.io from cluster", worker)
	if err := kc.KubectlDeleteResource("nodes.longhorn.io", worker, "-n", longhornNamespace); err != nil {
		return fmt.Errorf("error while deleting node %s from nodes.longhorn.io from cluster %s : %w", worker, d.clusterID, err)
	}

	return nil
}

// assureReplication tries to assure, that replicas for each Longhorn volume in nodes to be deleted
// are migrated to nodes which will remain in the cluster.
func (d *NodeDeleter) assureReplication(kc kubectl.Kubectl, workerNode string) error {
	// Get replicas and volumes as they can be scheduled on next node, which will be deleted.
	replicas, err := getReplicasMap(kc)
	if err != nil {
		return fmt.Errorf("error while getting replicas from cluster %s : %w", d.clusterID, err)
	}
	volumes, err := getVolumes(kc)
	if err != nil {
		return fmt.Errorf("error while getting volumes from cluster  %s : %w", d.clusterID, err)
	}
	if reps, ok := replicas[workerNode]; ok {
		for _, r := range reps {
			// Try to force creation of a new replicas from node, which will be deleted.
			if v, ok := volumes[r.Spec.VolumeName]; ok {
				// Increase number of replicas in volume.
				if err := increaseReplicaCount(v, kc); err != nil {
					return fmt.Errorf("error while increasing number of replicas in volume %s from cluster %s : %w", v.Metadata.Name, d.clusterID, err)
				}
				// Wait newReplicaCreationTimeout for Longhorn to create new replica.
				d.logger.Info().Msgf("Waiting %.0f seconds for new replicas to be scheduled if possible for node %s of cluster", newReplicaCreationTimeout.Seconds(), workerNode)
				time.Sleep(newReplicaCreationTimeout)

				// Verify all current replicas are running correctly
				if err := verifyAllReplicasSetUp(v.Metadata.Name, kc, d.logger); err != nil {
					return fmt.Errorf("error while checking if all longhorn replicas for volume %s are running : %w", v.Metadata.Name, err)
				}
				d.logger.Info().Msgf("Replication for volume %s has been set up", v.Metadata.Name)

				// Decrease number of replicas in volume -> original state.
				if err := revertReplicaCount(v, kc); err != nil {
					return fmt.Errorf("error while increasing number of replicas in volume %s cluster %s : %w", v.Metadata.Name, d.clusterID, err)
				}
				// Delete old replica, on to-be-deleted node.
				d.logger.Debug().Str("node", r.Status.OwnerID).Msgf("Deleting replica %s from node %s", r.Metadata.Name, r.Status.OwnerID)
				if err := deleteReplica(r, kc); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// updateClusterData will remove deleted nodes from nodepools.
func (d *NodeDeleter) updateClusterData() {
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
