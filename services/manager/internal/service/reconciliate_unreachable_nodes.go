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
	Hc          HealthCheckStatus
	Unreachable UnreachableNodes
	Diff        *KubernetesDiffResult
	Current     *spec.Clusters
	Desired     *spec.Clusters
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

		// the known unreachable infrastructure
		unreachableInfra = spec.Unreachable{
			Kubernetes: &spec.Unreachable_UnreachableNodePools{
				Nodepools: map[string]*spec.Unreachable_ListOfNodeEndpoints{},
			},
			Loadbalancers: map[string]*spec.Unreachable_UnreachableNodePools{},
		}
	)

	for np, endpoints := range r.Unreachable.Kubernetes {
		unreachableInfra.Kubernetes.Nodepools[np] = &spec.Unreachable_ListOfNodeEndpoints{
			Endpoints: slices.Clone(endpoints),
		}
	}

	for lb, nps := range r.Unreachable.LoadBalancers {
		unreachableInfra.Loadbalancers[lb] = &spec.Unreachable_UnreachableNodePools{
			Nodepools: map[string]*spec.Unreachable_ListOfNodeEndpoints{},
		}
		for np, endpoints := range nps {
			unreachableInfra.Loadbalancers[lb].Nodepools[np] = &spec.Unreachable_ListOfNodeEndpoints{
				Endpoints: slices.Clone(endpoints),
			}
		}
	}

	// Look at whole nodepools first.
	for np, ips := range r.Unreachable.Kubernetes {
		cnp := nodepools.FindByName(np, r.Current.K8S.ClusterInfo.NodePools)
		dnp := nodepools.FindByName(np, r.Desired.K8S.ClusterInfo.NodePools)

		if dnp == nil {
			if cnp.IsControl {
				controlpools := slices.Collect(nodepools.Control(r.Current.K8S.ClusterInfo.NodePools))
				if len(controlpools) == 1 {
					// Deleting last control plane nodepool will result in an invalid cluster.
					errUnreachable = errors.Join(
						errUnreachable,
						fmt.Errorf("can't delete nodepool %q with unreachable nodes, the "+
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
				Deleted: make(NodePoolsViewType, len(cnp.Nodes)),
			}

			for _, n := range cnp.Nodes {
				diff.Deleted[cnp.Name] = append(diff.Deleted[cnp.Name], n.Name)
			}

			next := ScheduleDeletionsInNodePools(r.Current, &diff, opts)
			return next, nil
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

			fmt.Fprintf(&errMsg, "node: %q, public endpoint: %q, static: %v;",
				cnp.Nodes[ci].Name,
				cnp.Nodes[ci].Public,
				cnp.GetStaticNodePool() != nil)
		}

		errMsg.WriteByte(']')

		errUnreachable = errors.Join(errUnreachable, fmt.Errorf("nodepool %q has %v unreachable kubernetes node/s: %s", np, len(ips), errMsg.String()))
	}

	if r.Hc.ApiEndpoint.Unreachable || len(r.Hc.Cluster.Nodes) == 0 {
		// We are not able to retrieve the actuall nodes within the kubernetes cluster.
		//
		// If this persists that means the control plane is down and there is nothing we can do from
		// claudie's POV. Deletion of the nodepools would also not help, essentially we are locked
		// until resolved manually.
		errUnreachable = fmt.Errorf(`
Failed to retrieve actuall nodes present in the cluster via 'kubectl'.

%v
`, errUnreachable)

		// Note: if the cluster has more than one control plane node
		// we should be able to recover by using the other control plane
		// nodes, however currently we do not have the structure for this.
		return nil, errUnreachable
	}

	// For any nodes/nodepools which have unreachable ips and the user did not remove
	// the whole nodepool from the desired state of the InputManifest, check if any of
	// the nodes were deleted manually from the cluster via `kubectl`.
	errUnreachable = nil
	for ip, info := range unreachableNodes {
		// node names inside k8s cluster have stripped cluster prefix.
		k8sname := strings.TrimPrefix(info.name, fmt.Sprintf("%s-", r.Current.K8S.ClusterInfo.Id()))

		if _, ok := r.Hc.Cluster.Nodes[k8sname]; ok {
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

		if info.static {
			// For the static nodes that were manually deleted they will also need to
			// be deleted from the desired state to not re-join the unreachable
			// static node again on the next iteration.
			np, node := nodepools.FindNode(r.Desired.K8S.ClusterInfo.NodePools, info.name)
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
		}

		logger.
			Info().
			Msgf(
				"Node %q from nodepool %q no longer part of the kubernetes cluster, will be scheduled for deletion",
				info.name,
				info.nodepool,
			)

		opts := K8sNodeDeletionOptions{
			UseProxy:     r.Diff.Proxy.CurrentUsed,
			HasApiServer: r.Diff.ApiEndpoint.Current != "",
			IsStatic:     info.static,
			Unreachable:  &unreachableInfra,
		}

		diff := NodePoolsDiffResult{
			PartiallyDeleted: NodePoolsViewType{
				info.nodepool: []string{info.name},
			},
		}

		next := ScheduleDeletionsInNodePools(r.Current, &diff, opts)
		return next, nil
	}

	if errUnreachable != nil {
		errUnreachable = fmt.Errorf(`
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
		return nil, errUnreachable
	}
	return nil, nil
}

type LoadBalancerUnreachableNodes struct {
	Unreachable UnreachableNodes
	Diff        *KubernetesDiffResult
	Current     *spec.Clusters
	Desired     *spec.Clusters
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

	for np, endpoints := range r.Unreachable.Kubernetes {
		unreachableInfra.Kubernetes.Nodepools[np] = &spec.Unreachable_ListOfNodeEndpoints{
			Endpoints: slices.Clone(endpoints),
		}
	}

	for lb, nps := range r.Unreachable.LoadBalancers {
		unreachableInfra.Loadbalancers[lb] = &spec.Unreachable_UnreachableNodePools{
			Nodepools: map[string]*spec.Unreachable_ListOfNodeEndpoints{},
		}
		for np, endpoints := range nps {
			unreachableInfra.Loadbalancers[lb].Nodepools[np] = &spec.Unreachable_ListOfNodeEndpoints{
				Endpoints: slices.Clone(endpoints),
			}
		}
	}

	for lb, unreachable := range r.Unreachable.LoadBalancers {
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
		errUnreachable = fmt.Errorf(`
Nodes within loadbalancers attached to the kubernetes cluster
have reachability problems.

Fix the unreachable nodes by either:
- fixing the connectivity issue
- if the connectivity issue cannot be resolved, you can:
  - delete the whole nodepool from the loadbalancer cluster in the InputManifest
  - delete the whole loadbalancer cluster from the InputManifest

%v
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
