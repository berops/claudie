package nodes

import (
	"encoding/json"
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

// etcdMemberList wraps parsed structures that are
// needed from the output of the `etcdctl member list`
// command, ignoring others.
type etcdMemberList struct {
	Members []struct {
		// Hex encoded id of the member within etcd.
		Id string

		// Name of the node within the cluster.
		Name string
	}
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
	keepNodePools map[string]struct{}

	logger zerolog.Logger
}

// New returns new Deleter struct, used for node deletion from a k8s cluster
// masterNodes - master nodes to DELETE
// workerNodes - worker nodes to DELETE
func NewDeleter(masterNodes, workerNodes []string, cluster *spec.K8Scluster, keepNodePools map[string]struct{}) *Deleter {
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
		keepNodePools: keepNodePools,
		logger:        loggerutils.WithClusterName(clusterID),
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
	etcdPods, err := getEtcdPodNames(kc, strings.TrimPrefix(etcdEpNode.Name, fmt.Sprintf("%s-", d.clusterPrefix)))
	if err != nil {
		return fmt.Errorf("cannot find etcd pods in cluster %s  : %w", d.clusterPrefix, err)
	}

	etcd, err := getEtcdMembers(kc, etcdPods[0])
	if err != nil {
		return fmt.Errorf("cannot find etcd members in cluster %s : %w", d.clusterPrefix, err)
	}

	for _, node := range d.masterNodes {
		found := false

		for _, member := range etcd.Members {
			if node.k8sName == member.Name {
				found = true
				d.logger.Debug().Msgf("Deleting etcd member %s, with hash %s", member.Name, member.Id)

				etcdctlCmd := fmt.Sprintf("etcdctl member remove %s", member.Id)
				if _, err := kc.KubectlExecEtcd(etcdPods[0], etcdctlCmd); err != nil {
					return fmt.Errorf("error while executing \"etcdctl member remove\" on node %s, cluster %s: %w", member.Name, d.clusterPrefix, err)
				}

				break
			}
		}

		if !found {
			d.logger.Warn().Msgf("%v is not a member of etcd, ignoring", node.k8sName)
		}
	}

	return nil
}

// updateClusterData will remove deleted nodes from nodepools
func (d *Deleter) updateClusterData() {
	for _, deleted := range append(d.masterNodes, d.workerNodes...) {
		d.cluster.ClusterInfo.NodePools = nodepools.DeleteNodeByName(d.cluster.ClusterInfo.NodePools, deleted.fullname, d.keepNodePools)
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

	var lines []string
	for line := range strings.SplitSeq(string(etcdPodsBytes), "\n") {
		if line := strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return nil, errors.New("no etcd pods found in cluster")
	}

	return lines, nil
}

// getEtcdMembers returns [etcdMemberList], each element containing etcd member info from "etcdctl member list"
func getEtcdMembers(kc kubectl.Kubectl, etcdPod string) (etcdMemberList, error) {
	var out etcdMemberList

	// List all members known by etcd, printed as a json with Hexified strings.
	cmd := "etcdctl member list -w json --hex=true"
	b, err := kc.KubectlExecEtcd(etcdPod, cmd)
	if err != nil {
		return out, fmt.Errorf("cannot find etcd members in cluster with etcd pod %s : %w", etcdPod, err)
	}

	if err := json.Unmarshal(b, &out); err != nil {
		return out, fmt.Errorf("failed to unmarshal etcd member list output: %w", err)
	}

	return out, nil
}
