package service

import (
	"errors"
	"fmt"
	"maps"
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

type KubernetesUnreachableNodes struct {
	Hc         HealthCheckStatus
	NodeStatus UnknownNodeStatus
	Diff       *KubernetesDiffResult
	Current    *spec.Clusters
	Desired    *spec.Clusters
}

// Based on the provided data via the [HealthCheckStatus] and [UnknownNodeStatus] determines what should be
// done to unblock the nodes with unknown status, the decision is returned via the (*[spec.TaskEvent], [error]) pair
//
// If an error is returned it means that the task cannot be scheduled due to needing input from the user as to
// what should be done next with the unreachable state.
//
// On no error there are two possible outcomes:
//   - A task is scheduled which should be worked on next with a higher priority to resolve the unreachable nodes.
//   - Nothing is returned (nil, nil), meaning that there is no task and also no error, the infrastructure is ok and reachable.
//
// If an [*spec.TaskEvent] is scheduled it does not point to or share any memory with the two passed in states.
func HandleKubernetesUnknownNodes(logger zerolog.Logger, r KubernetesUnreachableNodes) (*spec.TaskEvent, error) {
	unknownK8sNodes := make(map[string][]NodeDescription)

	// Merge both cases, unknown status, and nodes that were not
	// able to be joined into the cluster and consider them
	// as unreachable forcing to be reconciled in the future.
	maps.Copy(unknownK8sNodes, r.NodeStatus.UnknownKubernetesNodes)
	for k, v := range r.NodeStatus.NotJoinedKubernetesNodes {
		unknownK8sNodes[k] = append(unknownK8sNodes[k], v...)
	}

	var (
		// error that is gradually populated with the unreachable nodes info.
		errUnreachable error

		// the known unreachable infrastructure
		unreachableInfra = spec.Unreachable{
			Kubernetes: &spec.Unreachable_UnreachableNodePools{
				Nodepools: map[string]*spec.Unreachable_ListOfNodeEndpoints{},
			},
			Loadbalancers: map[string]*spec.Unreachable_UnreachableNodePools{},
		}
	)

	for np, nodes := range unknownK8sNodes {
		var endpoints []string

		for _, n := range nodes {
			endpoints = append(endpoints, n.PublicIPv4)
		}

		unreachableInfra.Kubernetes.Nodepools[np] = &spec.Unreachable_ListOfNodeEndpoints{
			Endpoints: slices.Clone(endpoints),
		}
	}

	for lb, nps := range r.NodeStatus.UnknownLoadBalancersNodes {
		unreachableInfra.Loadbalancers[lb] = &spec.Unreachable_UnreachableNodePools{
			Nodepools: map[string]*spec.Unreachable_ListOfNodeEndpoints{},
		}

		for np, endpoints := range nps {
			unreachableInfra.Loadbalancers[lb].Nodepools[np] = &spec.Unreachable_ListOfNodeEndpoints{
				Endpoints: slices.Clone(endpoints),
			}
		}
	}

	for np, nodes := range unknownK8sNodes {
		// cnp could be nil if a node is in the k8s cluster that
		// is not tracked by claudie.
		cnp := nodepools.FindByName(np, r.Current.K8S.ClusterInfo.NodePools)
		dnp := nodepools.FindByName(np, r.Desired.K8S.ClusterInfo.NodePools)

		if cnp != nil && dnp == nil {
			if cnp.IsControl {
				controlpools := slices.Collect(nodepools.Control(r.Current.K8S.ClusterInfo.NodePools))
				if len(controlpools) == 1 {
					// Deleting nodes from the last control plane nodepool will result in an invalid cluster.
					errUnreachable = errors.Join(
						errUnreachable,
						fmt.Errorf("can't delete nodes with unknown status from nodepool %q, the "+
							"nodepool is the last control nodepool, the deletion would result in a broken cluster",
							np,
						),
					)

					continue
				}

				if r.Diff.ApiEndpoint.Current != "" && cnp.EndpointNode() != nil {
					// deleted api endpoint in desired
					//
					// Note:
					//
					// In future, we could implement a "last resort" move of the api
					// endpoint and allow the the nodepool to be deleted, that would be
					// risky, but a way out of the deadlock.
					errUnreachable = errors.Join(
						errUnreachable,
						fmt.Errorf("can't delete nodepool %q with unknown node status, the "+
							"nodepool has the kubeapi server endpoint which would result in a broken cluster",
							np,
						),
					)

					continue
				}
			}

			opts := K8sNodeDeletionOptions{
				UseProxy:     r.Diff.Proxy.CurrentUsed,
				HasApiServer: r.Diff.ApiEndpoint.Current != "",
				IsStatic:     cnp.GetStaticNodePool() != nil,
				Unreachable:  &unreachableInfra,
			}

			diff := NodePoolsDiffResult{
				PartiallyDeleted: make(NodePoolsViewType, len(nodes)),
			}

			for _, n := range nodes {
				if n.IsStatic {
					diff.PartiallyDeleted[cnp.Name] = append(diff.PartiallyDeleted[cnp.Name], n.K8sName)
				} else {
					// k8s names have the cluster ID stripped.
					fullname := fmt.Sprintf("%s-%s", r.Current.K8S.ClusterInfo.Id(), n.K8sName)
					diff.PartiallyDeleted[cnp.Name] = append(diff.PartiallyDeleted[cnp.Name], fullname)
				}
			}

			logger.
				Info().
				Msgf(
					"Nodepool %q is scheduled for deletion as it has no desired state and nodes with unhealthy status", cnp.Name,
				)

			next := ScheduleDeletionsInNodePools(r.Current, &diff, opts)
			return next, nil
		}

		var canDelete []NodeDescription

		// Keep only nodes we can delete.
		for _, n := range nodes {
			if n.LastTransitionTime != nil && time.Since(n.LastTransitionTime.Time) <= TimeForNodeDeletion {
				continue
			}

			if n.IsStatic {
				nn, node := nodepools.FindNode(r.Desired.K8S.ClusterInfo.NodePools, n.K8sName)
				if node != nil && nn.GetStaticNodePool() != nil {
					if _, ok := r.Hc.Cluster.Nodes[n.K8sName]; ok {
						errUnreachable = errors.Join(
							errUnreachable,
							fmt.Errorf("\n - static node %q, nodepool %q, endpoint %q is unhealthy",
								n.K8sName,
								np,
								n.PublicIPv4,
							),
						)
					} else {
						errUnreachable = errors.Join(
							errUnreachable,
							fmt.Errorf(" - static node %q, nodepool %q, endpoint %q is unhealthy, "+
								"the node has been removed from the kubernetes cluster, to also remove "+
								"it from being tracked by Claudie remove it from the InputManifest",
								n.K8sName,
								np,
								n.PublicIPv4,
							),
						)
					}
					continue
				}

				// fallthrough
			}

			canDelete = append(canDelete, n)
		}

		if len(canDelete) < 1 {
			// Nothing to reconciliate, go to next nodepool.
			continue
		}

		var (
			diff = NodePoolsDiffResult{PartiallyDeleted: NodePoolsViewType{}}
			opts = K8sNodeDeletionOptions{
				UseProxy:     r.Diff.Proxy.CurrentUsed,
				HasApiServer: r.Diff.ApiEndpoint.Current != "",
				IsStatic:     canDelete[0].IsStatic,
				Unreachable:  &unreachableInfra,
			}
		)

		for _, n := range canDelete {
			var fullname string
			if n.IsStatic {
				fullname = n.K8sName
			} else {
				fullname = fmt.Sprintf("%s-%s", r.Current.K8S.ClusterInfo.Id(), n.K8sName)
			}
			diff.PartiallyDeleted[np] = append(diff.PartiallyDeleted[np], fullname)
		}

		logger.
			Info().
			Msgf(
				"Reconciling %d nodes from nodepool %q due to being unhealthy: %v",
				len(canDelete),
				np,
				slices.Collect(maps.Values(diff.PartiallyDeleted)),
			)

		next := ScheduleDeletionsInNodePools(r.Current, &diff, opts)
		return next, nil
	}

	if errUnreachable != nil {
		// nolint
		errUnreachable = fmt.Errorf(`
The following nodes within the kubernetes cluster have reachability problems:

%w

Fix the unreachable nodes by either:
- fixing the connectivity issue
- if the connectivity issue cannot be resolved, you can:
 - delete the whole nodepool from the kubernetes cluster in the InputManifest
 - delete the selected unreachable node/s manually from the cluster via 'kubectl delete node ...'
 - delete the node from the InputManifest, if it is a static node.

   NOTE: if the unreachable node is the kube-apiserver, claudie will not be able to recover
         after the deletion.
`, errUnreachable)
		return nil, errUnreachable
	}

	return nil, nil
}

type LoadBalancerUnreachableNodes struct {
	NodeStatus UnknownNodeStatus
	Diff       *KubernetesDiffResult
	Current    *spec.Clusters
	Desired    *spec.Clusters
}

// Similar as [HandleKubernetesUnreachableNodes] but works with the loadbalancer nodes.
func HandleLoadBalancerUnknownNodes(r LoadBalancerUnreachableNodes) (*spec.TaskEvent, error) {
	// for each loadbalancer check if the nodepool with the unreachable nodes is present in the desired
	// state. If not issue a delete, if yes wait for either the connectivity issue to be resolved or the
	// removal of the nodepool from the desired state.
	//
	// We do not allow deleting of a single node from a loadbalancer nodepool at this time, as it is the
	// case for kuberentes nodes.
	var (
		// error that is gradually populated with the unreachable nodes info.
		errUnreachable error

		// the known unreachable infrastructure
		unreachableInfra = spec.Unreachable{
			Kubernetes: &spec.Unreachable_UnreachableNodePools{
				Nodepools: map[string]*spec.Unreachable_ListOfNodeEndpoints{},
			},
			Loadbalancers: map[string]*spec.Unreachable_UnreachableNodePools{},
		}
	)

	// Merge both cases, unknown status, and nodes that were not
	// able to be joined into the cluster and consider them
	// as unreachable forcing to be reconciled in the future.
	unknownK8sNodes := make(map[string][]NodeDescription)
	maps.Copy(unknownK8sNodes, r.NodeStatus.UnknownKubernetesNodes)
	for k, v := range r.NodeStatus.NotJoinedKubernetesNodes {
		unknownK8sNodes[k] = append(unknownK8sNodes[k], v...)
	}

	for np, d := range unknownK8sNodes {
		var endpoints []string
		for _, n := range d {
			endpoints = append(endpoints, n.PublicIPv4)
		}

		unreachableInfra.Kubernetes.Nodepools[np] = &spec.Unreachable_ListOfNodeEndpoints{
			Endpoints: endpoints,
		}
	}

	for lb, nps := range r.NodeStatus.UnknownLoadBalancersNodes {
		unreachableInfra.Loadbalancers[lb] = &spec.Unreachable_UnreachableNodePools{
			Nodepools: map[string]*spec.Unreachable_ListOfNodeEndpoints{},
		}
		for np, endpoints := range nps {
			unreachableInfra.Loadbalancers[lb].Nodepools[np] = &spec.Unreachable_ListOfNodeEndpoints{
				Endpoints: slices.Clone(endpoints),
			}
		}
	}

	for lb, unreachable := range r.NodeStatus.UnknownLoadBalancersNodes {
		cid := clusters.IndexLoadbalancerById(lb, r.Current.LoadBalancers.Clusters)
		did := clusters.IndexLoadbalancerById(lb, r.Desired.LoadBalancers.Clusters)

		clb := r.Current.LoadBalancers.Clusters[cid]
		if did < 0 {
			// cluster with the unreachable ips has been deleted from the desired state.
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
					fmt.Errorf("can't delete loadbalancer %q containing nodes with unknown status, the loadbalancer is targeting"+
						" the kube-api server and deleting it from the desired state would result in a broken kubernetes cluster", lb),
				)
				continue
			}

			id := LoadBalancerIdentifier{
				Id:    lb,
				Index: cid,
			}

			// If any other parts of the infra is unresponsive this task will not be blocked by it.
			next := ScheduleRawDeleteLoadBalancer(r.Current, id, &unreachableInfra)
			return next, nil
		}

		dlb := r.Desired.LoadBalancers.Clusters[did]
		for np, ips := range unreachable {
			cnp := nodepools.FindByName(np, clb.ClusterInfo.NodePools)
			dnp := nodepools.FindByName(np, dlb.ClusterInfo.NodePools)

			// nodepool with bad ips was deleted from the desired state.
			if dnp == nil {
				opts := LoadBalancerNodePoolsOptions{
					UseProxy:    r.Diff.Proxy.CurrentUsed,
					IsStatic:    cnp.GetStaticNodePool() != nil,
					Unreachable: &unreachableInfra,
				}

				diff := NodePoolsDiffResult{
					Deleted: make(NodePoolsViewType, len(cnp.Nodes)),
				}
				for _, n := range cnp.Nodes {
					diff.Deleted[cnp.Name] = append(diff.Deleted[cnp.Name], n.Name)
				}

				next := ScheduleDeletionLoadBalancerNodePools(
					r.Current,
					LoadBalancerIdentifier{
						Id:    lb,
						Index: cid,
					},
					&diff,
					opts,
				)
				return next, nil
			}

			// Note:
			//
			// in future this could delete the unreachable nodes
			// as the loadbalancers has a manadatory DNS
			// record which points to the VMs. the DNS
			// wouldn't be deleted just updated with 0 nodes
			// and then again updated with newly reconciled
			// nodes.
			//
			// However for this to work we need to track for how
			// long a loadbalancer is unresponsive for, which currently
			// is not.
			//
			// Need to also consider the fact where claudie will not
			// have internet access and no schedule a unwanted removal
			// as a result of that.

			errMsg := strings.Builder{}
			errMsg.WriteString("[")
			for _, ip := range ips {
				ci := slices.IndexFunc(cnp.Nodes, func(n *spec.Node) bool { return n.Public == ip })
				fmt.Fprintf(&errMsg, "node: %q, public endpoint: %q, static: %v;",
					cnp.Nodes[ci].Name,
					cnp.Nodes[ci].Public,
					cnp.GetStaticNodePool() != nil,
				)
			}
			errMsg.WriteByte(']')
			errUnreachable = errors.Join(
				errUnreachable,
				fmt.Errorf("nodepool %q from loadbalancer %q has %v loadbalancer node/s with unknown status:\n %s", np, lb, len(ips), errMsg.String()),
			)
		}
	}

	if errUnreachable != nil {
		// nolint
		errUnreachable = fmt.Errorf(`
Node/s within loadbalancers attached to the kubernetes cluster
have reachability problems.

Fix the unreachable nodes by either:
- fixing the connectivity issue
- if the connectivity issue cannot be resolved, you can:
  - delete the whole nodepool from the loadbalancer cluster in the InputManifest
  - delete the whole loadbalancer cluster from the InputManifest

%w
`, errUnreachable)
		return nil, errUnreachable
	}
	return nil, nil
}

// Deletes the loadbalancer with the id specified in the passed in lb from the [spec.Clusters] state. The deletion
// Only deletes the Infrastructure if any. Contrary to how the [ScheduleDeleteLoadBalancer] works this function will
// not run/execute any other stages as part of its delete pipeline. This task is useful for scenarios where the loadbalancer
// infrastructure is unresponsive and needs to be replaced. The ansibler stage will not be called at all, thus there
// should be other mechanisms in place to reconcile the ansible stage.
//
// The returned [spec.TaskEvent] does not point to or share any memory with the two passed in states.
func ScheduleRawDeleteLoadBalancer(
	current *spec.Clusters,
	cid LoadBalancerIdentifier,
	unreachable *spec.Unreachable,
) *spec.TaskEvent {
	inFlight := proto.Clone(current).(*spec.Clusters)
	updateOp := spec.Update{
		State: &spec.Update_State{
			K8S:           inFlight.K8S,
			LoadBalancers: inFlight.LoadBalancers.Clusters,
		},
		Delta: &spec.Update_DeleteLoadBalancer_{
			DeleteLoadBalancer: &spec.Update_DeleteLoadBalancer{
				Handle:      cid.Id,
				Unreachable: unreachable,
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
