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
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
)

const (
	longhornNamespace = "longhorn-system"

	UnreachableNodesPingCount = clusters.PingRetryCount + 3
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
	kubeconfig    string
	clusterPrefix string
	controlNode   string
}

// New returns new Deleter struct, used for node deletion from a k8s cluster
// The passed in [spec.K8Scluster] is not modified in any way by any of the
// functions of the deleter. Simply the passed in masterNodes and workerNodes
// are being worked with to delete them from the kubernetes cluster via the
// kubeconfig of the [spec.K8Scluster].
func NewDeleter(
	deleteMaster, deleteWorker []string,
	cluster *spec.K8Scluster,
) (*Deleter, error) {
	var (
		clusterID = cluster.ClusterInfo.Id()
		mn, wn    []nodeInfo
	)

	for i := range deleteMaster {
		mn = append(mn, nodeInfo{
			fullname:       deleteMaster[i],
			k8sName:        strings.TrimPrefix(deleteMaster[i], fmt.Sprintf("%s-", clusterID)),
			publicEndpoint: clusters.NodePublic(deleteMaster[i], cluster),
		})
	}

	for i := range deleteWorker {
		wn = append(wn, nodeInfo{
			fullname:       deleteWorker[i],
			k8sName:        strings.TrimPrefix(deleteWorker[i], fmt.Sprintf("%s-", clusterID)),
			publicEndpoint: clusters.NodePublic(deleteWorker[i], cluster),
		})
	}

	// find a control node that will not be deleted.
	var (
		controlNodes = slices.Collect(nodepools.Control(cluster.ClusterInfo.NodePools))
		notDeleted   string
	)

	for _, cn := range controlNodes {
		if !slices.Contains(deleteMaster, cn.Name) {
			notDeleted = cn.Name
			break
		}
	}

	if notDeleted == "" {
		return nil, fmt.Errorf(
			"out of the %v control nodes, after the deletion none will remain, invalid state", len(controlNodes),
		)
	}

	return &Deleter{
		masterNodes:   mn,
		workerNodes:   wn,
		kubeconfig:    cluster.Kubeconfig,
		clusterPrefix: clusterID,
		controlNode:   strings.TrimPrefix(notDeleted, fmt.Sprintf("%s-", clusterID)),
	}, nil
}

// DeleteNodes deletes nodes specified in d.masterNodes and d.workerNodes
// return nil if successful, error otherwise
func (d *Deleter) DeleteNodes(logger zerolog.Logger) error {
	kubectl := kubectl.Kubectl{
		Kubeconfig:        d.kubeconfig,
		MaxKubectlRetries: 5,
	}
	kubectl.Stdout = comm.GetStdOut(d.clusterPrefix)
	kubectl.Stderr = comm.GetStdErr(d.clusterPrefix)

	// get real node names
	n, err := kubectl.KubectlGetNodeNames()
	if err != nil {
		return fmt.Errorf("error while getting nodes from cluster: %w", err)
	}

	var k8snodes []string
	for node := range strings.SplitSeq(string(n), "\n") {
		if node := strings.TrimSpace(node); node != "" {
			k8snodes = append(k8snodes, node)
		}
	}
	var errDel error

	// Remove master nodes sequentially to minimise risk of faults in etcd
	for _, master := range d.masterNodes {
		if !slices.Contains(k8snodes, master.k8sName) {
			logger.
				Warn().
				Msgf("Node with name %s not found in cluster", master.k8sName)
			continue
		}

		logger.
			Info().
			Msgf("verifying if node %s is reachable", master.k8sName)

		if err := clusters.Ping(logger, UnreachableNodesPingCount, master.publicEndpoint); err != nil {
			if errors.Is(err, clusters.ErrEchoTimeout) {
				logger.
					Info().
					Msgf(
						"Node %s is unreachable, marking node with `out-of-service` taint "+
							"before deleting it from the cluster", master.k8sName,
					)

				if err := kubectl.KubectlTaintNodeShutdown(master.k8sName); err != nil {
					logger.
						Err(err).
						Msgf(
							"Failed to taint node %s with 'out-of-service' taint, proceeding with default deletion",
							master.k8sName,
						)
				}
			} else {
				logger.
					Err(err).
					Msgf(
						"Failed to determine if node %s is reachable or not, proceeding with default deletion",
						master.k8sName,
					)
			}
		}

		// IMPORTANT: first you have to cordon and drain, and only after that you can remove from etcd
		// kubectl cordon <node-name> <args>
		if err := kubectl.KubectlCordon(master.k8sName); err != nil {
			errDel = errors.Join(errDel, fmt.Errorf("error while cordon master node %s from cluster: %w", master.k8sName, err))
			continue
		}

		// kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data
		if err := kubectl.KubectlDrain(master.k8sName); err != nil {
			return fmt.Errorf("error while draining node %s from cluster: %w", master.k8sName, err)
		}

		// delete master nodes from etcd
		if err := d.deleteFromEtcd(logger, kubectl); err != nil {
			return fmt.Errorf("error while deleting nodes from etcd: %w", err)
		}

		// delete master nodes
		if err := d.deleteNodesByName(logger, kubectl, master, k8snodes); err != nil {
			return fmt.Errorf("error while deleting nodes from master nodes: %w", err)
		}
	}

	// Remove worker nodes sequentially to minimise risk of fault when replicating PVC
	for _, worker := range d.workerNodes {
		if !slices.Contains(k8snodes, worker.k8sName) {
			logger.
				Warn().
				Msgf("Node with name %s not found in cluster", worker.k8sName)
			continue
		}

		logger.
			Info().
			Msgf("verifying if node %s is reachable", worker.k8sName)

		if err := clusters.Ping(logger, UnreachableNodesPingCount, worker.publicEndpoint); err != nil {
			if errors.Is(err, clusters.ErrEchoTimeout) {
				logger.
					Info().
					Msgf(
						"Node %s is unreachable, marking node with `out-of-service` taint "+
							"before deleting it from the cluster", worker.k8sName,
					)

				if err := kubectl.KubectlTaintNodeShutdown(worker.k8sName); err != nil {
					logger.
						Err(err).
						Msgf(
							"Failed to taint node %s with 'out-of-service' taint, proceeding with default deletion",
							worker.k8sName,
						)
				}
			} else {
				logger.
					Err(err).
					Msgf(
						"Failed to determine if node %s is reachable or not, proceeding with default deletion",
						worker.k8sName,
					)
			}
		}

		// kubectl cordon <node-name> <args>
		if err := kubectl.KubectlCordon(worker.k8sName); err != nil {
			errDel = errors.Join(errDel, fmt.Errorf("error while cordon worker node %s from cluster: %w", worker.k8sName, err))
			continue
		}

		// kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data
		if err := kubectl.KubectlDrain(worker.k8sName); err != nil {
			return fmt.Errorf("error while draining node %s from cluster: %w", worker.k8sName, err)
		}

		if err := d.deleteNodesByName(logger, kubectl, worker, k8snodes); err != nil {
			errDel = errors.Join(errDel, fmt.Errorf("error while deleting node %s from cluster: %w", worker.k8sName, err))
			continue
		}

		if err := removeReplicasOnDeletedNode(kubectl, worker.k8sName); err != nil {
			// not a fatal error.
			logger.
				Warn().
				Msgf("failed to delete unused replicas from replicas.longhorn.io, after node %s deletion: %s", worker.k8sName, err)
		}
	}

	return errDel
}

// deleteNodesByName deletes node from the k8s cluster.
func (d *Deleter) deleteNodesByName(logger zerolog.Logger, kc kubectl.Kubectl, node nodeInfo, realNodeNames []string) error {
	if !slices.Contains(realNodeNames, node.k8sName) {
		logger.Warn().Msgf("Node with name %s not found in cluster", node.k8sName)
		return nil
	}

	logger.Info().Msgf("Deleting node %s from k8s cluster", node.k8sName)

	//kubectl delete node <node-name>
	if err := kc.KubectlDeleteResource("nodes", node.k8sName); err != nil {
		return fmt.Errorf("error while deleting node %s from cluster: %w", node.k8sName, err)
	}

	return nil
}

// deleteFromEtcd function deletes members of the etcd cluster. This needs to be done in order to prevent any data corruption in etcd
// return nil if successful, error otherwise
func (d *Deleter) deleteFromEtcd(logger zerolog.Logger, kc kubectl.Kubectl) error {
	etcdPods, err := getEtcdPodNames(kc, d.controlNode)
	if err != nil {
		return fmt.Errorf("cannot find etcd pods in cluster: %w", err)
	}

	etcd, err := getEtcdMembers(kc, etcdPods[0])
	if err != nil {
		return fmt.Errorf("cannot find etcd members in cluster: %w", err)
	}

	for _, node := range d.masterNodes {
		found := false

		for _, member := range etcd.Members {
			if node.k8sName == member.Name {
				found = true
				logger.Debug().Msgf("Deleting etcd member %s, with hash %s", member.Name, member.Id)

				etcdctlCmd := fmt.Sprintf("member remove %s", member.Id)
				if _, err := kc.KubectlExecEtcd(etcdPods[0], etcdctlCmd); err != nil {
					return fmt.Errorf("error while executing \"etcdctl member remove\" on node %s, cluster: %w", member.Name, err)
				}

				break
			}
		}

		if !found {
			logger.Warn().Msgf("%v is not a member of etcd, ignoring", node.k8sName)
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
	cmd := "member list -w json --hex=true"
	b, err := kc.KubectlExecEtcd(etcdPod, cmd)
	if err != nil {
		return out, fmt.Errorf("cannot find etcd members in cluster with etcd pod %s : %w", etcdPod, err)
	}

	if err := json.Unmarshal(b, &out); err != nil {
		return out, fmt.Errorf("failed to unmarshal etcd member list output: %w", err)
	}

	return out, nil
}
