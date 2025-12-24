package service

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ScheduleUnreachableNodesResult describes the changes made within
// one of the [HandleKubernetesUnreachableNodes] or [HandleLoadBalancerUnreachableNodes]
// function.
type ScheduleUnreachableNodesResult uint8

const (
	// Unreachable Nodes Noop signals that no operation was needed, meaning that
	// all of the current state is reachable by the management cluster.
	//
	// No changes to the passed in state was done in this case.
	UnreachableNodesNoop ScheduleUnreachableNodesResult = iota

	// Unreachable Nodes Scheduled Task specifies that a task was scheduled
	// which addresses part or the whole issue with the unreachable nodes.
	//
	// Any task that was alreaady in part of the current state is overwritten
	// and this newly scheduled task should be worked on with a higher priority.
	UnreachableNodesScheduledTask

	// Unreachable Nodes Modified Current State specifies that the current state
	// was modified and that any indices into it were invalidated and should no
	// longer be considered valid.
	//
	// The current state is modified mostly related to handling unrechable static
	// nodes to which no pipeline needs to be executed as they're unreachable and
	// they're simply 'forgot' from the current state.
	//
	// Before any new changes/Task are to be worked, should be processed after
	// propagating the the current making the change persistent.
	//
	// Any existing scheduled task is removed.
	UnreachableNodesModifiedCurrentState

	// Unreachable Nodes Propagate Error specifies that the error status with a
	// descriptive error message was set in the [spec.ClusterState.State] which
	// should be propagated to the user.
	//
	// No changes to the passed in state was done in this case.
	//
	// Any existing scheduled task is removed.
	UnreachableNodesPropagateError
)

func (s ScheduleUnreachableNodesResult) String() string {
	switch s {
	case UnreachableNodesModifiedCurrentState:
		return "UnreachableNodesModifiedCurrentState"
	case UnreachableNodesNoop:
		return "UnreachableNodesNoop"
	case UnreachableNodesPropagateError:
		return "UnreachableNodesPropagateError"
	case UnreachableNodesScheduledTask:
		return "UnreachableNodesScheduledTask"
	default:
		return ""
	}
}

// Based on the provided data via the [HealthCheckStatus] and [UnreachableIPv4Map] determines what should be
// done to unblock the unreachable nodes, the decision is returned via the [ScheduleUnreachableNodesResult].
//
// **This function makes changes to the current state, thus any indexes will be invalidated**
//
// If an [*spec.TaskEvent] is scheduled it does not point to or share any memory with the two passed in states.
func HandleKubernetesUnreachableNodes(
	logger zerolog.Logger,
	hc HealthCheckStatus,
	nps UnreachableIPv4Map,
	state *spec.ClusterState,
	desired *spec.Clusters,
) ScheduleUnreachableNodesResult {
	// Check if the nodepools with unreachability are present in the desired
	// state. We then need to check if they are present in the k8s cluster,
	// and based on that make a decision about what to do with the nodes.
	type unreachableNodeInfo struct {
		nodepool string
		name     string
		static   bool
	}

	var (
		// Cache for unreachable nodes data that were not deleted in the desired state.
		unreachableNodes = make(map[string]unreachableNodeInfo)

		// error that is gradually populated with the unreachable nodes info.
		errUnreachable error
	)

	// Look at whole nodepools first.
	for np, ips := range nps {
		cnp := nodepools.FindByName(np, state.Current.K8S.ClusterInfo.NodePools)
		dnp := nodepools.FindByName(np, desired.K8S.ClusterInfo.NodePools)

		if dnp == nil {
			if cnp.EndpointNode() != nil {
				// deleted api endpoint in desired
				//
				// Note:
				//
				// In future, we could implement a "last resort" move of the api
				// endpoint and allow the the nodepool to be deleted, that would be
				// risky, but a way out of the deadlock.
				errUnreachable = errors.Join(
					errUnreachable,
					fmt.Errorf("can't delete nodepool %q with unreachable nodes, the "+
						"nodepool has the kubeapi server endpoint which would result in a broken cluster",
						np,
					),
				)

				continue
			}

			switch cnp.Type.(type) {
			case *spec.NodePool_DynamicNodePool:
				var nodes []string
				for _, n := range cnp.Nodes {
					nodes = append(nodes, n.Name)
				}

				next := ScheduleRawDeletionNodePool(current, true, np, nodes)
				state.InFlight = next
				return UnreachableNodesScheduledTask
			case *spec.NodePool_StaticNodePool:
				// Unreachable static nodepools don't have to be deleted, simply
				// remove them from the current state. The vpn will be refreshed
				// by the healthcheck or by the next workflow run, along with the
				// proxy settings if any.
				state.Current.K8S.ClusterInfo.NodePools = nodepools.DeleteByName(
					state.Current.K8S.ClusterInfo.NodePools,
					np,
				)
				state.InFlight = nil
				return UnreachableNodesModifiedCurrentState
			default:
				logger.Error().Msgf("%q unrecognized nodepool type %T, ignoring healthcheck handling until resolved", np, cnp.Type)
				continue
			}
		}

		errMsg := strings.Builder{}
		errMsg.WriteByte('[')

		for _, ip := range ips {
			ci := slices.IndexFunc(cnp.Nodes, func(n *spec.Node) bool { return n.Public == ip })

			unreachableNodes[ip] = unreachableNodeInfo{
				name:     cnp.Nodes[ci].Name,
				static:   cnp.GetStaticNodePool() != nil,
				nodepool: np,
			}

			errMsg.WriteString(fmt.Sprintf("node: %q, public endpoint: %q, static: %v;",
				cnp.Nodes[ci].Name,
				cnp.Nodes[ci].Public,
				cnp.GetStaticNodePool() != nil,
			))
		}

		errMsg.WriteByte(']')

		errUnreachable = errors.Join(errUnreachable, fmt.Errorf("nodepool %q has %v unreachable kubernetes node/s: %s", np, len(ips), errMsg.String()))
	}

	if hc.ApiEndpoint.Unreachable || len(hc.Cluster.Nodes) == 0 {
		// We are not able to retrieve the actuall nodes within the kubernetes cluster.
		//
		// If this persists that means the control plane is down and there is nothing we can do from
		// claudie's POV. Deletion of the nodepools would also not help, essentially we are locked
		// until resolved manually.
		state.InFlight = nil
		state.State.Status = spec.Workflow_ERROR
		state.State.Description = fmt.Sprintf(`
Failed to retrieve actuall nodes present in the cluster via 'kubectl'.

%v
`, errUnreachable)

		logger.Error().Msg(state.State.Description)

		// TODO: if the cluster has more than one control plane node
		// we should be able to recover by using the other control plane
		// nodes, however currently we do not have the structure for this.
		return UnreachableNodesPropagateError
	}

	// For any nodes/nodepools which have unreachable ips and the user did not remove
	// the whole nodepool from the desired state of the InputManifest, check if any of
	// the nodes were deleted manually from the cluster via `kubectl`.
	errUnreachable = nil
	for ip, info := range unreachableNodes {
		// node names inside k8s cluster have stripped cluster prefix.
		k8sname := strings.TrimPrefix(info.name, fmt.Sprintf("%s-", state.Current.K8S.ClusterInfo.Id()))

		if _, ok := hc.Cluster.Nodes[k8sname]; ok {
			// unreachable node is still in the cluster.

			errUnreachable = errors.Join(
				errUnreachable,
				fmt.Errorf(" - node: %q, nodepool %q, public endpoint: %q, static: %v",
					info.name,
					info.nodepool,
					ip,
					info.static,
				),
			)

			continue
		}

		// node is no longer in the cluster, issue a delete of
		// the infrastructure.
		if info.static {
			// For the static nodes that were manually they will also need to
			// be deleted from the desired state to not re-join the unreachable
			// static node again on the next iteration.
			np, node := nodepools.FindNode(desired.K8S.ClusterInfo.NodePools, info.name)
			if node != nil && np.GetStaticNodePool() != nil {
				errUnreachable = errors.Join(
					errUnreachable,
					fmt.Errorf(" - detected that static node %q, nodepool %q, endpoint %q was "+
						"removed from the kubernetes cluster, remove the static node from the "+
						"InputManifest as well to avoid re-joining it again",
						info.name,
						info.nodepool,
						ip,
					),
				)
				continue
			}

			// The vpn will be refreshed by the healthcheck or by
			// the next workflow run, along with the proxy settings
			// if any.
			state.InFlight = nil
			nodepools.DeleteNodes(np, []string{info.name})

			logger.
				Info().
				Msgf(
					"Static node %q from nodepool %q no longer part of the kubernetes cluster, will be scheduled for deletion",
					info.name,
					info.nodepool,
				)

			return UnreachableNodesModifiedCurrentState
		} else {
			// state.InFlight = next...
			panic("todo")
			// return schedule deletion.

			logger.
				Info().
				Msgf("Node %q from nodepool %q no longer part of the kubernetes cluster, will be scheduled for deletion",
					info.name,
					info.nodepool,
				)

			return UnreachableNodesScheduledTask
		}

	}

	if errUnreachable != nil {
		state.InFlight = nil
		state.State.Status = spec.Workflow_ERROR
		state.State.Description = fmt.Sprintf(`
Nodes within the kubernetes cluster have reachability problems.

Fix the unreachable nodes by either:
- fixing the connectivity issue
- if the connectivity issue cannot be resolved, you can:
 - delete the whole nodepool from the kubernetes cluster in the InputManifest
 - delete the selected unreachable node/s manually from the cluster via 'kubectl'
   - if its a static node you will also need to remove it from the InputManifest
   - if its a dynamic node claudie will replace it.
   NOTE: if the unreachable node is the kube-apiserver, claudie will not be able to recover
         after the deletion.

%v
`, errUnreachable)

		logger.Error().Msg(state.State.Description)
		return UnreachableNodesPropagateError
	}

	return UnreachableNodesNoop
}

// Similar as [HandleKubernetesUnreachableNodes] but work with the loadbalancer nodes.
func HandleLoadBalancerUnreachableNodes(
	logger zerolog.Logger,
	unreachable map[string]UnreachableIPv4Map,
	state *spec.ClusterState,
	desired *spec.Clusters,
) ScheduleUnreachableNodesResult {
	// for each loadbalancer check if the nodepool with the unreachable nodes is present in the desired
	// state. If not issue a delete, if yes wait for either the connectivity issue to be resolved or the
	// removal of the nodepool from the desired state.
	//
	// We do not allow deleting of a single node from a loadbalancer nodepool at this time, as it is the
	// case for kuberentes nodes.
	var errUnreachable error

	for lb, unreachable := range unreachable {
		cid := clusters.IndexLoadbalancerById(lb, state.Current.LoadBalancers.Clusters)
		did := clusters.IndexLoadbalancerById(lb, desired.LoadBalancers.Clusters)

		clb := state.Current.LoadBalancers.Clusters[cid]
		if did < 0 {
			// cluster with the unrechable ips has been deleted from the desired state.
			if clb.IsApiEndpoint() {
				// deleted api endpoint in desired
				//
				// Note:
				//
				// In future, we could implement a "last resort" move of the api
				// endpoint and allow the loadbalancers to delete, that would be risky
				// but a way out of the deadlock.
				errUnreachable = errors.Join(
					errUnreachable,
					fmt.Errorf("can't delete loadbalancer %q with unreachable nodes, the loadbalancer is targeting"+
						" the kube-api server and deleting it from the desired state would result in a broken kubernetes cluster", lb),
				)

				continue
			}

			id := LoadBalancerIdentifier{
				Id:    lb,
				Index: cid,
			}

			// If any other parts of the infra is unresponsive this task will not be blocked by it.
			next := ScheduleRawDeleteLoadBalancer(state.Current, id)
			state.InFlight = next
			return UnreachableNodesScheduledTask
		}

		dlb := desired.LoadBalancers.Clusters[did]
		for np, ips := range unreachable {
			cnp := nodepools.FindByName(np, clb.ClusterInfo.NodePools)
			dnp := nodepools.FindByName(np, dlb.ClusterInfo.NodePools)

			// nodepool with bad ips was deleted from the desired state.
			if dnp == nil {
				switch cnp.Type.(type) {
				case *spec.NodePool_DynamicNodePool:
					var nodes []string
					for _, n := range cnp.Nodes {
						nodes = append(nodes, n.Name)
					}

					next := ScheduleRawDeletionLoadBalancerNodePool(
						state.Current,
						LoadBalancerIdentifier{
							Id:    lb,
							Index: cid,
						},
						np,
						nodes,
					)
					state.InFlight = next
					return UnreachableNodesScheduledTask
				case *spec.NodePool_StaticNodePool:
					// Unreachable Static nodepools don't have to be deleted,
					// simply 'forget' them from the current state.
					//
					// The vpn will be refreshed by the healthcheck or by
					// the next workflow run, along with the proxy settings
					// if any.
					state.Current.K8S.ClusterInfo.NodePools = nodepools.DeleteByName(state.Current.K8S.ClusterInfo.NodePools, np)
					state.InFlight = nil
					return UnreachableNodesModifiedCurrentState
				default:
					logger.Error().Msgf("%q unrecognized nodepool type %T, ignoring healthcheck handling until resolved", np, cnp.Type)
					continue
				}
			}

			errMsg := strings.Builder{}
			errMsg.WriteString("[")
			for _, ip := range ips {
				ci := slices.IndexFunc(cnp.Nodes, func(n *spec.Node) bool { return n.Public == ip })
				errMsg.WriteString(fmt.Sprintf("node: %q, public endpoint: %q, static: %v;",
					cnp.Nodes[ci].Name,
					cnp.Nodes[ci].Public,
					cnp.GetStaticNodePool() != nil,
				))
			}
			errMsg.WriteByte(']')
			errUnreachable = errors.Join(
				errUnreachable,
				fmt.Errorf("nodepool %q from loadbalancer %q has %v unreachable loadbalancer node/s: %s", np, lb, len(ips), errMsg.String()),
			)
		}
	}

	if errUnreachable != nil {
		state.InFlight = nil
		state.State.Status = spec.Workflow_ERROR
		state.State.Description = fmt.Sprintf(`
Nodes within loadbalancers attached to the kubernetes clsuter
have reachability problems.

Fix the unreachable nodes by either:
- fixing the connectivity issue
- if the connectivity issue cannot be resolved, you can:
  - delete the whole nodepool from the loadbalancer cluster in the InputManifest
  - delete the whole loadbalancer cluster from the InputManifest

%v
`, errUnreachable)

		return UnreachableNodesPropagateError
	}

	return UnreachableNodesNoop
}

type RawNodePoolDeletionOptions struct {
	// Whether the kubernetes cluster has the Api server.
	HasApiServer bool

	// If the whole nodepool is deleted.
	WholeNodePoolDeleted bool
}

// Schedules a task that will delete nodes/nodepools from the current state of the cluster.
// Contrary to how the [ScheduleDeletionsInNodePools] works, this will only remove the nodes
// from the kubernetes cluster and then destroy then terraform infrastructure, if any. No
// other stages will be scheduled. This task is usefull for removing unreachable state without
// depending on the reachability of other nodes within the cluster.
//
// **Only works for dynamic nodepools**
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleRawDeletionNodePool(
	current *spec.Clusters,
	nodepool string, nodes []string,
	opts RawNodePoolDeletionOptions,
) *spec.TaskEvent {
	// TODO: I think there should be a new message called Raw deletion, that will take
	// into account and filter the invalid nodes from the ping and only run ansible on
	// those nodes ?
	//
	// I think these "Scheduled" Tasks should be in the reconciliate_unreachable_nodes
	// file and that it should follow the same path as the stantard deletion but with
	// the filtering of know unreachable nodes both in loadbalancers and kuberentes
	// clusters, but the task will only remove one thing at a time from the cluster.
	//
	// Thah should move the state gradually to an okay state.
	inFlight := proto.Clone(current).(*spec.Clusters)

	kuber := spec.Stage_Kuber{
		Kuber: &spec.StageKuber{
			Description: &spec.StageDescription{
				About:      "Reconciling cluster configuration",
				ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
			},
			SubPasses: []*spec.StageKuber_SubPass{
				{
					Kind: spec.StageKuber_DELETE_NODES,
					Description: &spec.StageDescription{
						About:      "Deleting nodes from kubernetes cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
				},
			},
		},
	}

	// The healthcheck within the reconciliation loop will trigger a refresh of the VPN.
	pipeline := []*spec.Stage{
		{
			StageKind: &kuber,
		},
		{
			StageKind: &spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Reconciling infrastructure for kubernetes cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      "Destroying VMs for deleted nodes",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		},
	}

	if opts.WholeNodePoolDeleted {
		// If the ApiServer is on the kubernetes cluster on deletion of the
		// control plane nodes the Kubeadm config map needs to be updated which
		// is used during the join operation of new nodes.
		if opts.HasApiServer && nodepools.FindByName(nodepool, current.K8S.ClusterInfo.NodePools).IsControl {
			kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, []*spec.StageKuber_SubPass{
				{
					Kind: spec.StageKuber_PATCH_KUBEADM,
					Description: &spec.StageDescription{
						About:      "Updating Kubeadm certSANs",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
				},
				{
					Kind: spec.StageKuber_CILIUM_RESTART,
					Description: &spec.StageDescription{
						About: "Rollout restart of cilium pods",
						// Rollout restart failing is not a fatal error.
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
				},
			}...)
		}

		// If the deletion of the last autoscaled nodepools is to be scheduled. Also remove
		// the CA requirement for the cluster.
		if a := nodepools.Autoscaled(current.K8S.ClusterInfo.NodePools); len(a) == 1 {
			if a[0].Name == nodepool {
				kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, &spec.StageKuber_SubPass{
					Kind: spec.StageKuber_DISABLE_LONGHORN_CA,
					Description: &spec.StageDescription{
						About:      "Disabling Longhorn cluster autoscaler setting",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
				})
			}
		}

		// On deletion of the whole nodepool, reconcile the storage classes for longhorn.
		kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, &spec.StageKuber_SubPass{
			Kind: spec.StageKuber_RECONCILE_LONGHORN_STORAGE_CLASSES,
			Description: &spec.StageDescription{
				About:      "Reconciling claudie longhorn storage classes after nodepool deletion",
				ErrorLevel: spec.ErrorLevel_ERROR_WARN,
			},
		})

		// If changes to the nodepool affects any loadbalancer
		// also schedule a reconciliation of loadbalancers, as
		// the envoy targets needs to be regenerated.
		for _, lb := range current.LoadBalancers.Clusters {
			for _, r := range lb.Roles {
				for _, tg := range r.TargetPools {
					// Need to match againts only the nodepool name without the hash.
					if n, _ := nodepools.MatchNameAndHashWithTemplate(tg, np); n != "" {
						ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, &spec.StageAnsibler_SubPass{
							Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
							Description: &spec.StageDescription{
								About:      "Reconciling Envoy settings after changes to nodepool",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						})
					}
				}
			}
		}

		return &spec.TaskEvent{
			Id:        uuid.New().String(),
			Timestamp: timestamppb.New(time.Now().UTC()),
			Event:     spec.Event_UPDATE,
			Task: &spec.Task{
				Do: &spec.Task_Update{
					Update: &spec.Update{
						State: &spec.Update_State{
							K8S:           inFlight.K8S,
							LoadBalancers: inFlight.LoadBalancers.Clusters,
						},
						Delta: &spec.Update_DeleteK8SNodes_{
							DeleteK8SNodes: &spec.Update_DeleteK8SNodes{
								Nodepool:     nodepool,
								Nodes:        nodes,
								WithNodePool: true,
							},
						},
					},
				},
			},
			Description: fmt.Sprintf("Deleting nodepool %s", nodepool),
			Pipeline:    pipeline,
		}
	}

	for np, nodes := range diff.PartiallyDeleted {
		// If the ApiServer is on the kubernetes cluster on deletion of the
		// control plane nodes the Kubeadm config map needs to be updated which
		// is used during the join operation of new nodes.
		if opts.HasApiServer && nodepools.FindByName(np, current.K8S.ClusterInfo.NodePools).IsControl {
			kuber.Kuber.SubPasses = append(kuber.Kuber.SubPasses, []*spec.StageKuber_SubPass{
				{
					Kind: spec.StageKuber_PATCH_KUBEADM,
					Description: &spec.StageDescription{
						About:      "Updating Kubeadm certSANs",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
				},
				{
					Kind: spec.StageKuber_CILIUM_RESTART,
					Description: &spec.StageDescription{
						About: "Rollout restart of cilium pods",
						// Rollout restart failing is not a fatal error.
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
				},
			}...)
		}

		// If changes to the nodepool affects any loadbalancer
		// also schedule a reconciliation of loadbalancers, as
		// the envoy targets needs to be regenerated.
		for _, lb := range current.LoadBalancers.Clusters {
			for _, r := range lb.Roles {
				for _, tg := range r.TargetPools {
					// Need to match againts only the nodepool name without the hash.
					if n, _ := nodepools.MatchNameAndHashWithTemplate(tg, np); n != "" {
						ans.Ansibler.SubPasses = append(ans.Ansibler.SubPasses, &spec.StageAnsibler_SubPass{
							Kind: spec.StageAnsibler_RECONCILE_LOADBALANCERS,
							Description: &spec.StageDescription{
								About:      "Reconciling Envoy settings after changes to nodepool",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						})
					}
				}
			}
		}

		return &spec.TaskEvent{
			Id:        uuid.New().String(),
			Timestamp: timestamppb.New(time.Now().UTC()),
			Event:     spec.Event_UPDATE,
			Task: &spec.Task{
				Do: &spec.Task_Update{
					Update: &spec.Update{
						State: &spec.Update_State{
							K8S:           inFlight.K8S,
							LoadBalancers: inFlight.LoadBalancers.Clusters,
						},
						Delta: &spec.Update_DeleteK8SNodes_{
							DeleteK8SNodes: &spec.Update_DeleteK8SNodes{
								Nodepool:     np,
								Nodes:        nodes,
								WithNodePool: false,
							},
						},
					},
				},
			},
			Description: fmt.Sprintf("Deleting %v nodes from nodepool %s", len(nodes), np),
			Pipeline:    pipeline,
		}
	}

	return nil
}

// Deletes the loadbalancer with the id specified in the passed in lb from the [spec.Clusters] state. The deletion
// Only deletes the Infrastructure if any. Contrary to how the [ScheduleDeleteLoadBalancer] works this function will
// not run/execute any other stages as part of its delete pipeline. This task is usefull for scenarios where the loadbalancer
// infrastructure is unresponsive and needs to be replaced.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleRawDeleteLoadBalancer(current *spec.Clusters, cid LoadBalancerIdentifier) *spec.TaskEvent {
	inFlight := proto.Clone(current).(*spec.Clusters)
	updateOp := spec.Update{
		State: &spec.Update_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.Update_DeleteLoadBalancer_{
			DeleteLoadBalancer: &spec.Update_DeleteLoadBalancer{
				Handle: cid.Id,
			},
		},
	}

	// The healthcheck within the reconciliation loop will trigger a refresh of the VPN.
	pipeline := []*spec.Stage{
		{
			StageKind: &spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Reconciling infrastructure",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      "Destroying infrastructure",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		},
	}

	// If we are deleting the last loadbalancer delete scrape config.
	// Try to delete the scrape config, if any, on failure ignore all
	// errors.
	if len(current.LoadBalancers.Clusters) == 1 {
		pipeline = append(pipeline, &spec.Stage{
			StageKind: &spec.Stage_Kuber{
				Kuber: &spec.StageKuber{
					Description: &spec.StageDescription{
						About:      "Configuring cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
					SubPasses: []*spec.StageKuber_SubPass{
						{
							Kind: spec.StageKuber_REMOVE_LB_SCRAPE_CONFIG,
							Description: &spec.StageDescription{
								About:      "Removing load balancer scrape config",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN,
							},
						},
					},
				},
			},
		})
	}

	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_UPDATE,
		Task: &spec.Task{
			Do: &spec.Task_Update{
				Update: &updateOp,
			},
		},
		Description: fmt.Sprintf("Removing load balancer %q", cid.Id),
		Pipeline:    pipeline,
	}
}

// Schedules a task that will delete a nodepool from the current state of the loadbalancer.
// Contrary to how the [ScheduleDeletionLoadBalancerNodePools] task works, this will only
// run the terraformer stage for the destruction of the unreachable infrastructure without
// running any other stages. This task is usefull when needing to remove unreachable state
// from the cluster.
//
// **Only works for dynamic Nodepools**
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleRawDeletionLoadBalancerNodePool(
	current *spec.Clusters,
	cid LoadBalancerIdentifier,
	nodepool string,
	nodes []string, // all of the nodes of the deleted nodepool.
) *spec.TaskEvent {
	pipeline := []*spec.Stage{
		{
			StageKind: &spec.Stage_Terraformer{
				Terraformer: &spec.StageTerraformer{
					Description: &spec.StageDescription{
						About:      "Reconciling infrastructure for the load balancer",
						ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
					},
					SubPasses: []*spec.StageTerraformer_SubPass{
						{
							Kind: spec.StageTerraformer_UPDATE_INFRASTRUCTURE,
							Description: &spec.StageDescription{
								About:      "Remvoing firewalls and nodes",
								ErrorLevel: spec.ErrorLevel_ERROR_FATAL,
							},
						},
					},
				},
			},
		},

		// try to update the scrape config, on any error, simply ignore them.
		{
			StageKind: &spec.Stage_Kuber{
				Kuber: &spec.StageKuber{
					Description: &spec.StageDescription{
						About:      "Configuring kubernetes cluster",
						ErrorLevel: spec.ErrorLevel_ERROR_WARN,
					},
					SubPasses: []*spec.StageKuber_SubPass{
						{
							Kind: spec.StageKuber_STORE_LB_SCRAPE_CONFIG,
							Description: &spec.StageDescription{
								About:      "Reconciling scrape config",
								ErrorLevel: spec.ErrorLevel_ERROR_WARN,
							},
						},
					},
				},
			},
		},
	}

	inFlight := proto.Clone(current).(*spec.Clusters)
	return &spec.TaskEvent{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(time.Now().UTC()),
		Event:     spec.Event_UPDATE,
		Task: &spec.Task{
			Do: &spec.Task_Update{
				Update: &spec.Update{
					State: &spec.Update_State{
						K8S:           inFlight.K8S,
						LoadBalancers: inFlight.LoadBalancers.Clusters,
					},
					Delta: &spec.Update_DeleteLoadBalancerNodes_{
						DeleteLoadBalancerNodes: &spec.Update_DeleteLoadBalancerNodes{
							Handle:       cid.Id,
							WithNodePool: true,
							Nodepool:     nodepool,
							Nodes:        nodes,
						},
					},
				},
			},
		},
		Description: fmt.Sprintf("Deleting nodepool %q from load balancer %q", nodepool, cid.Id),
		Pipeline:    pipeline,
	}
}
