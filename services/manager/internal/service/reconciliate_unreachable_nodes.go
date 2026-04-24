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

type KubernetesUnreachableNodes struct {
	Hc         HealthCheckStatus
	NodeStatus UnknownNodeStatus
	Diff       *KubernetesDiffResult
	Current    *spec.Clusters
	Desired    *spec.Clusters
}

// Based on the provided data via the [HealthCheckStatus] and [UnreachableNodes] determines what should be
// done to unblock the unreachable nodes, the decision is returned via the (*[spec.TaskEvent], [error]) pair
//
// If an error is returned it means that the task cannot be scheduled due to needing input from the user as to
// what should be done next with the unreachable state.
//
// On no error there are two possible outcomes:
//   - A task is scheduled which should be worked on next with a higher priority to resolve the unreachable nodes.
//   - Nothing is returned (nil, nil), meaning that there is no task and also no error, the infrastructure is ok and reachable.
//
// If an [*spec.TaskEvent] is scheduled it does not point to or share any memory with the two passed in states.
func HandleKubernetesUnreachableNodes(logger zerolog.Logger, r KubernetesUnreachableNodes) (*spec.TaskEvent, error) {
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

	for np, nodes := range r.NodeStatus.UnknownKubernetesNodes {
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

	for np, nodes := range r.NodeStatus.UnknownKubernetesNodes {
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
						fmt.Errorf("can't delete unreachable nodes from nodepool %q, the "+
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
						fmt.Errorf("can't delete nodepool %q with unreachable nodes, the "+
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

			next := ScheduleDeletionsInNodePools(r.Current, &diff, opts)
			return next, nil
		}

		for _, n := range nodes {
			fullname := fmt.Sprintf("%s-%s", r.Current.K8S.ClusterInfo.Id(), n.K8sName)

			if n.LastTransitionTime != nil && time.Since(n.LastTransitionTime.Time) <= TimeForNodeDeletion {
				continue
			}

			if n.IsStatic {
				// static nodes do not have the cluster prefix.
				fullname = n.K8sName

				nn, node := nodepools.FindNode(r.Desired.K8S.ClusterInfo.NodePools, fullname)
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

			if n.IsStatic {
				logger.Info().Msgf("Removing node %q from the cluster", fullname)
			} else {
				logger.
					Info().
					Msgf("Replacing node %q in the cluster, as it was unhealthy for more than %s", fullname, TimeForNodeDeletion)
			}

			opts := K8sNodeDeletionOptions{
				UseProxy:     r.Diff.Proxy.CurrentUsed,
				HasApiServer: r.Diff.ApiEndpoint.Current != "",
				IsStatic:     n.IsStatic,
				Unreachable:  &unreachableInfra,
			}

			diff := NodePoolsDiffResult{
				PartiallyDeleted: NodePoolsViewType{
					np: []string{fullname},
				},
			}

			next := ScheduleDeletionsInNodePools(r.Current, &diff, opts)
			return next, nil
		}
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
func HandleLoadBalancerUnreachableNodes(r LoadBalancerUnreachableNodes) (*spec.TaskEvent, error) {
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

	for np, d := range r.NodeStatus.UnknownKubernetesNodes {
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
				fmt.Errorf("nodepool %q from loadbalancer %q has %v unreachable loadbalancer node/s:\n %s", np, lb, len(ips), errMsg.String()),
			)
		}
	}

	if errUnreachable != nil {
		// nolint
		errUnreachable = fmt.Errorf(`
Nodes within loadbalancers attached to the kubernetes cluster
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
