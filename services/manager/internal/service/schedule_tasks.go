package service

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func scheduleTasks(scheduled *store.Config) (ScheduleResult, error) {
	for cluster, state := range scheduledGRPC.Clusters {
		logger := loggerutils.WithProjectAndCluster(scheduledGRPC.Name, cluster)

		var events []*spec.TaskEvent
		switch {
		default:
			logger.Info().Msg("verifying if all nodes in the current state are reachable")
			k8sip, lbsip, err := clusters.PingNodes(logger, state.Current)
			if err != nil {
				if errors.Is(err, clusters.ErrEchoTimeout) {
					if len(k8sip) > 0 {
						e, apply := tryReachK8sNodes(logger, k8sip, state)
						if !apply {
							result = NotReady
							break
						}
						events = append(events, e...)
					}
					if len(lbsip) > 0 {
						e, apply := tryReachLbNodes(logger, lbsip, state)
						if !apply {
							result = NotReady
							break
						}
						events = append(events, e...)
					}
					result = Reschedule
				} else {
					logger.Err(err).Msg("failed to determine if any nodes were unreachable")
					result = NotReady
				}
				break
			}

			if state.State.Status == spec.Workflow_ERROR {
				if tasks := state.Events.Events; len(tasks) != 0 && tasks[0].OnError.Do != nil {
					task := tasks[0]

					switch s := task.OnError.Do.(type) {
					case *spec.Retry_Repeat_:
						events = tasks

						if s.Repeat.Kind == spec.Retry_Repeat_EXPONENTIAL {
							if s.Repeat.RetryAfter > 0 {
								s.Repeat.RetryAfter--
								result = NotReady
								break
							}

							s.Repeat.CurrentTick <<= 1
							if s.Repeat.CurrentTick >= s.Repeat.StopAfter {
								// final retry before error-ing out.
								result = FinalRetry
								task.OnError.Do = nil
								break
							}

							s.Repeat.RetryAfter = s.Repeat.CurrentTick
						}

						result = Reschedule
						logger.Debug().Msgf("rescheduled for a retry of previously failed task with ID %q.", task.Id)
					case *spec.Retry_Rollback_:
						result = Reschedule
						events = s.Rollback.Tasks
						logger.Debug().Msgf("rescheduled for a rollback with task ID %q of previous failed task with ID %q.", events[0].Id, task.Id)
					default:
						result = NoReschedule
						logger.Debug().Msgf("has not been rescheduled for a retry on failure")
					}

					if result == Reschedule || result == NotReady || result == FinalRetry {
						break
					}
				}
			}
		}
	}
}

func tryReachLbNodes(logger zerolog.Logger, lbs map[string]map[string][]string, state *spec.ClusterState) (events []*spec.TaskEvent, apply bool) {
	// for each loadbalancer check if the nodepool with the unreachable nodes is present in the desired
	// state. If not issue a delete, if yes wait for either the connectivity issue to be resolved or the
	// removal of the nodepool from the desired state.
	// We do not allow deleting of a single node from a loadbalancer nodepool at this time, as it is the
	// case for kuberentes nodes.
	var (
		errUnreachable error
		toDelete       = make(map[string]*spec.DeleteState_LoadBalancer)
	)

	for lb, unreachable := range lbs {
		toDelete[lb] = &spec.DeleteState_LoadBalancer{
			Id:        lb,
			Nodepools: make(map[string]*spec.DeletedNodes),
		}

		ci := clusters.IndexLoadbalancerById(lb, state.Current.GetLoadBalancers().GetClusters())
		di := clusters.IndexLoadbalancerById(lb, state.Desired.GetLoadBalancers().GetClusters())

		current := state.Current.GetLoadBalancers().GetClusters()[ci]

		if di < 0 {
			// cluster with the unrechable ips has been deleted from the desired state.
			if current.IsApiEndpoint() {
				// deleted api endpoint in desired
				errUnreachable = errors.Join(errUnreachable, fmt.Errorf("can't delete loadbalancer %q with unreachable nodes, the loadbalancer is targeting the kube-api server and deleting it from the desired state would result in a broken kubernetes cluster", lb))
			}
			toDelete[lb].Destroy = true
			continue
		}

		desired := state.Desired.GetLoadBalancers().GetClusters()[di]
		for np, ips := range unreachable {
			current := nodepools.FindByName(np, current.GetClusterInfo().GetNodePools())
			desired := nodepools.FindByName(np, desired.GetClusterInfo().GetNodePools())

			if desired == nil {
				// nodepool with bad ips was deleted from the desired state.
				dn := new(spec.DeletedNodes)
				for _, n := range current.Nodes {
					dn.Nodes = append(dn.Nodes, n.Name)
				}
				toDelete[lb].Nodepools[np] = dn
				continue
			}

			errMsg := strings.Builder{}
			errMsg.WriteString("[")
			for _, ip := range ips {
				ci := slices.IndexFunc(current.Nodes, func(n *spec.Node) bool { return n.Public == ip })
				errMsg.WriteString(fmt.Sprintf("node: %q, public endpoint: %q, static: %v;",
					current.Nodes[ci].Name,
					current.Nodes[ci].Public,
					current.GetStaticNodePool() != nil,
				))
			}
			errMsg.WriteByte(']')
			errUnreachable = errors.Join(errUnreachable, fmt.Errorf("nodepool %q from loadbalancer %q has %v unreachable loadbalancer node/s: %s", np, lb, len(ips), errMsg.String()))
		}
	}

	if errUnreachable != nil {
		state.State.Description = fmt.Sprintf(`%v

fix the unreachable nodes by either:
- fixing the connectivity issue
- if the connectivity issue cannot be resolved, you can:
  - delete the whole nodepool from the loadbalancer cluster in the InputManifest
  - delete the whole loadbalancer cluster from the InputManifest
`, errUnreachable)
		logger.Warn().Msgf("%v", state.State.Description)
		return
	}

	events = append(events, &spec.TaskEvent{
		Id:          uuid.New().String(),
		Timestamp:   timestamppb.New(time.Now().UTC()),
		Event:       spec.Event_DELETE,
		Description: "deleting unreachable nodes from k8s cluster",
		Task:        &spec.Task{DeleteState: &spec.DeleteState{Lbs: slices.Collect(maps.Values(toDelete))}},
	})
	apply = true
	return
}

// tryReachK8sNodes determines if the InputManifest should be rescheduled or not based on the desired state and the reachability of the
// kubernetes nodes of the cluster. If the InputManifest is not ready to be scheduled yet apply will be false. Only if apply is true
// will the function also returns events that need to be handled before any other.
func tryReachK8sNodes(logger zerolog.Logger, nps map[string][]string, state *spec.ClusterState) (events []*spec.TaskEvent, apply bool) {
	// Check if the nodepools with unreachability are present in the desired state.
	// We then need to check if they are present in the k8s cluster, and based on that make a
	// decision about what to do with the nodes.
	type unreachableNodeInfo struct {
		name   string
		static bool
	}
	var (
		unreachableNodes = make(map[string]unreachableNodeInfo)
		toDelete         = make(map[string]*spec.DeletedNodes)
		errUnreachable   error
	)

	for np, ips := range nps {
		current := nodepools.FindByName(np, state.Current.GetK8S().GetClusterInfo().GetNodePools())
		desired := nodepools.FindByName(np, state.Desired.GetK8S().GetClusterInfo().GetNodePools())

		if desired == nil {
			toDelete[np] = new(spec.DeletedNodes)
			for _, n := range current.Nodes {
				toDelete[np].Nodes = append(toDelete[np].Nodes, n.Name)
			}
		}

		errMsg := strings.Builder{}
		errMsg.WriteByte('[')
		for _, ip := range ips {
			ci := slices.IndexFunc(current.Nodes, func(n *spec.Node) bool { return n.Public == ip })
			unreachableNodes[ip] = unreachableNodeInfo{
				name:   current.Nodes[ci].Name,
				static: current.GetStaticNodePool() != nil,
			}
			errMsg.WriteString(fmt.Sprintf("node: %q, public endpoint: %q, static: %v;",
				current.Nodes[ci].Name,
				current.Nodes[ci].Public,
				current.GetStaticNodePool() != nil,
			))
		}
		errMsg.WriteByte(']')

		errUnreachable = errors.Join(errUnreachable, fmt.Errorf("nodepool %q has %v unreachable kubernetes node/s: %s", np, len(ips), errMsg.String()))
	}

	state.State = &spec.Workflow{
		Stage:  spec.Workflow_NONE,
		Status: spec.Workflow_ERROR,
		// Description: will be filled based on what action needs to be done.
	}

	kubectl := kubectl.Kubectl{
		Kubeconfig:        state.Current.K8S.Kubeconfig,
		MaxKubectlRetries: 5,
	}

	// TODO: if the cluster has more than one control plane node
	// we should be able to recover by using the other control plane
	// nodes, however currently we do not have the structure for this.
	n, err := kubectl.KubectlGetNodeNames()
	if err != nil {
		state.State.Description = fmt.Sprintf("%v\nfailed to retrieve actual nodes present in the cluster via 'kubectl': %v", errUnreachable, err)
		logger.Err(err).Msgf("failed to retrieve actuall nodes present in the cluster via `kubectl`, retrying later\n%v", errUnreachable)
		// We are not able to retrieve the actuall nodes within the kubernetes cluster.
		// If this persists that means the control plane is down and there is nothing we can do from
		// claudie's POV. Deletion of the nodepools would also not help, essentially we are locked
		// until resolved manually.
		return
	}

	nodesInCluster := make(map[string]struct{})
	for n := range strings.SplitSeq(string(n), "\n") {
		nodesInCluster[n] = struct{}{}
	}

	// ignore the nodepools that were deleted in the desired state.
	for np := range toDelete {
		delete(nps, np)
	}

	// For any nodepools which have unreachable ips and the user did not remove the
	// nodepool from the desired state of the InputManifest, check if any of the nodes
	// were deleted manually from the cluster via 'kubectl'
	errUnreachable = nil
	for np, ips := range nps {
		fix := 0
		errMsg := strings.Builder{}

		for _, ip := range ips {
			info := unreachableNodes[ip]
			// node names inside k8s cluster have stripped cluster prefix.
			k8sname := strings.TrimPrefix(info.name, fmt.Sprintf("%s-", state.Current.GetK8S().GetClusterInfo().Id()))
			if _, ok := nodesInCluster[k8sname]; ok {
				// unreachable node is still in the cluster.
				errMsg.WriteString(fmt.Sprintf(" - node: %q, public endpoint: %q, static: %v", info.name, ip, info.static))
				fix++
				continue
			}
			if _, ok := toDelete[np]; !ok {
				toDelete[np] = new(spec.DeletedNodes)
			}
			toDelete[np].Nodes = append(toDelete[np].Nodes, info.name)

			// For the nodes that were manually deleted check which of them are static nodes
			// as they will also need to be deleted from the desired state to not re-join the
			// unreachable static node again on the next iteration.
			static, node := nodepools.FindNode(state.GetDesired().GetK8S().GetClusterInfo().GetNodePools(), info.name)
			if node != nil && static {
				fix++
				errMsg.WriteString(
					fmt.Sprintf(" - detected that static node %q with endpoint %q from nodepool %q was removed from the kubernetes cluster, remove the static node from the desired state by adjusting the InputManifest.",
						info.name,
						node.Public,
						np,
					),
				)
				continue
			}
			logger.Info().Msgf("node %q from nodepool %q no longer part of the kubernetes cluster, will be scheduled for deletion", info.name, np)
		}

		if fix > 0 {
			errUnreachable = errors.Join(errUnreachable, fmt.Errorf("\nnodepool %q has %v unreachable kubernetes node/s that need to be fixed:\n%s",
				np,
				fix,
				errMsg.String(),
			))
		}
	}

	// If the user did not delete the unreachable nodes via kubectl or the user
	// did not remove the whole nodepool with the unreachable nodes from the
	// desired state we cannot proceed further as we need to remove all the
	// nodes with connectivity issue in one go. We cannot issue partial
	// removal as the workflow will get stuck in ansibler which connects
	// to the nodes via ssh.
	if errUnreachable != nil {
		state.State.Description = fmt.Sprintf(`%v

fix the unreachable nodes by either:
- fixing the connectivity issue
- if the connectivity issue cannot be resolved, you can:
  - delete the whole nodepool from the kubernetes cluster in the InputManifest
  - delete the selected unreachable node/s manually from the cluster via 'kubectl'
    - if its a static node you will also need to remove it from the InputManifest
    - if its a dynamic node claudie will replace it.
    NOTE: if the unreachable node is the kube-apiserver, claudie will not be able to recover
          after the deletion.
`, errUnreachable)
		// neither deletion in the desired state
		// nor deletion in the kubernetes cluster
		// has been done. nothing to do.
		logger.Warn().Msgf("%v", state.State.Description)
		return
	}

	events = append(events, &spec.TaskEvent{
		Id:          uuid.New().String(),
		Timestamp:   timestamppb.New(time.Now().UTC()),
		Event:       spec.Event_DELETE,
		Description: "deleting unreachable nodes from k8s cluster",
		Task:        &spec.Task{DeleteState: &spec.DeleteState{K8S: &spec.DeleteState_K8S{Nodepools: toDelete}}},
	})
	apply = true
	return
}
