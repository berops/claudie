package service

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// rollingUpdate compares the commit hashes between the templates of the current state, desired state and
// the latest commit of the template repository. Returns an Intermediate state where the nodepools for which
// an updated commit hashes was found, are replaced with new nodepools and events that need to be executed
// by the builder to achieve the "desired" intermediate state.
// The rolling update of nodepools works as follows:
//
//  1. A clone of the current state is made.
//
//  2. On this clone the commit hash of the requested template repositories are updated to reflect the latest
//     changes. If the template repository uses a tag the commit hash will not change.
//
//  3. The desired state is then used to overwrite changes made by the user in this intermediate state.
//     (catching cases where the user changed the tag, or disabled the tag)
//
//  4. Each nodepool in the current state is then compared to the respective nodepool in the new intermediate state.
//     On different commit hashes a rolling update will be performed.
func rollingUpdate(current, desired *spec.Clusters) (*spec.Clusters, []*spec.TaskEvent, error) {
	var (
		events            []*spec.TaskEvent
		mapping           = make(map[int]*spec.NodePool)
		usedNodePoolNames = make(map[string]struct{})
		ir                = proto.Clone(current.K8S).(*spec.K8Scluster)
		rollingUpdates    = proto.Clone(current.K8S).(*spec.K8Scluster)
		k8sID             = ir.ClusterInfo.Id()
	)

	maps.Insert(usedNodePoolNames, maps.All(nodepoolNames(current.K8S.ClusterInfo.NodePools)))
	maps.Insert(usedNodePoolNames, maps.All(nodepoolNames(desired.K8S.ClusterInfo.NodePools)))

	if err := syncWithRemoteRepo(ir.ClusterInfo.NodePools); err != nil {
		return nil, nil, err
	}

	transferTemplatesRepo(ir.ClusterInfo.NodePools, desired.K8S.ClusterInfo.NodePools)

	for di, d := range ir.ClusterInfo.NodePools {
		desiredPool := d.GetDynamicNodePool()
		if desiredPool == nil {
			continue
		}

		ci := slices.IndexFunc(rollingUpdates.ClusterInfo.NodePools, func(c *spec.NodePool) bool { return c.Name == d.Name })
		currentPool := rollingUpdates.ClusterInfo.NodePools[ci]
		if currentPool.GetDynamicNodePool().Provider.Templates.CommitHash == desiredPool.Provider.Templates.CommitHash {
			continue
		}

		updated := proto.Clone(currentPool).(*spec.NodePool)
		mapping[di] = updated

		// 1. new name
		n, _ := utils.MustExtractNameAndHash(d.Name)
		for {
			name := fmt.Sprintf("%s-%s", n, utils.CreateHash(utils.HashLength))
			if _, ok := usedNodePoolNames[name]; !ok {
				usedNodePoolNames[name] = struct{}{}
				updated.Name = name
				break
			}
		}

		// 2. new keys/cidr
		var err error
		updatedDyn := updated.GetDynamicNodePool()
		updatedDyn.Cidr = ""
		if updatedDyn.PublicKey, updatedDyn.PrivateKey, err = generateSSHKeyPair(); err != nil {
			return nil, nil, err
		}

		// 3. replace provider
		updatedDyn.Provider = desiredPool.Provider

		// 4. new nodes
		updated.Nodes = nil
		nodeNames := make(map[string]struct{})
		nodepoolID := fmt.Sprintf("%s-%s", k8sID, updated.Name)
		generateMissingDynamicNodes(nodepoolID, nodeNames, updated)

		rollback := proto.Clone(rollingUpdates).(*spec.K8Scluster) // clone in case of failure to rollback to.

		rollingUpdates.ClusterInfo.NodePools = append(rollingUpdates.ClusterInfo.NodePools, updated)
		addNodePool := proto.Clone(rollingUpdates).(*spec.K8Scluster) // clone as the cluster will gradually change.

		rollingUpdates.ClusterInfo.NodePools = slices.Delete(rollingUpdates.ClusterInfo.NodePools, ci, ci+1)
		delNodePool := proto.Clone(rollingUpdates).(*spec.K8Scluster) // clone as the cluster will gradually change.

		// add new nodepool.
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: fmt.Sprintf("rolling update: replacing %s with %s", currentPool.Name, updated.Name),
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: addNodePool,
					Lbs: current.LoadBalancers, // keep current lbs
				},
			},
			OnError: &spec.Retry{Do: &spec.Retry_Rollback_{Rollback: &spec.Retry_Rollback{
				Tasks: []*spec.TaskEvent{
					{
						Id:          uuid.New().String(),
						Timestamp:   timestamppb.New(time.Now().UTC()),
						Event:       spec.Event_DELETE,
						Description: fmt.Sprintf("rollback: deleting nodes from replaced nodepool %s", updated.Name),
						Task: &spec.Task{DeleteState: &spec.DeleteState{
							Nodepools: map[string]*spec.DeletedNodes{
								updated.Name: {
									Nodes: func() []string {
										var result []string
										for _, n := range updated.Nodes {
											result = append(result, n.Name)
										}
										return result
									}(),
								},
							},
						}},
						OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
							Kind: spec.Retry_Repeat_ENDLESS,
						}}},
					},
					{
						Id:          uuid.New().String(),
						Timestamp:   timestamppb.New(time.Now().UTC()),
						Event:       spec.Event_UPDATE,
						Description: fmt.Sprintf("rollback: deleting infrastructure of deleted nodes from nodepool %s", updated.Name),
						Task: &spec.Task{
							UpdateState: &spec.UpdateState{
								K8S: rollback,
								Lbs: current.LoadBalancers, // keep current lbs
							},
						},
						OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
							Kind: spec.Retry_Repeat_ENDLESS,
						}}},
					},
				},
			}}},
		})

		log.Debug().
			Str("cluster", k8sID).
			Msgf("created event %q with Rollback on error [%q, %q] with repeat on rollback failure",
				events[len(events)-1].Description,
				events[len(events)-1].OnError.Do.(*spec.Retry_Rollback_).Rollback.Tasks[0].Description,
				events[len(events)-1].OnError.Do.(*spec.Retry_Rollback_).Rollback.Tasks[1].Description,
			)

		// delete nodes from old nodepool.
		var deletedApiEndpoint bool
		var delNodes []string
		for _, n := range currentPool.Nodes {
			if n.NodeType == spec.NodeType_apiEndpoint {
				deletedApiEndpoint = true
			}
			delNodes = append(delNodes, n.Name)
		}

		// transfer API endpoint if needed.
		if deletedApiEndpoint {
			events = append(events, &spec.TaskEvent{
				Id:          uuid.New().String(),
				Timestamp:   timestamppb.New(time.Now().UTC()),
				Event:       spec.Event_UPDATE,
				Description: fmt.Sprintf("rolling update: moving endpoint from old control plane node to a new control plane node %q from nodepool %q", updated.Nodes[0].Name, updated.Name),
				Task: &spec.Task{
					UpdateState: &spec.UpdateState{Endpoint: &spec.UpdateState_Endpoint{
						Nodepool: updated.Name,
						Node:     updated.Nodes[0].Name,
					}},
				},
				OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
					Kind: spec.Retry_Repeat_ENDLESS,
				}}},
			})

			log.Debug().
				Str("cluster", k8sID).
				Msgf("created event %q with Repeat on error", events[len(events)-1].Description)
		}

		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_DELETE,
			Description: fmt.Sprintf("rolling update: deleting nodes from replaced nodepool %s", currentPool.Name),
			Task: &spec.Task{DeleteState: &spec.DeleteState{
				Nodepools: map[string]*spec.DeletedNodes{
					currentPool.Name: {Nodes: delNodes},
				},
			}},
			OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
				Kind: spec.Retry_Repeat_ENDLESS,
			}}},
		})

		log.Debug().
			Str("cluster", k8sID).
			Msgf("created event %q with Repeat on error", events[len(events)-1].Description)

		// delete infra from old nodepool.
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: fmt.Sprintf("rolling update: deleting infrastructure of deleted nodes from nodepool %s", currentPool.Name),
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: delNodePool,
					Lbs: current.LoadBalancers, // keep current lbs
				},
			},
			OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
				Kind: spec.Retry_Repeat_ENDLESS,
			}}},
		})

		log.Debug().
			Str("cluster", k8sID).
			Msgf("created event %q with Repeat on error", events[len(events)-1].Description)
	}

	for di, updated := range mapping {
		ir.ClusterInfo.NodePools[di] = updated
	}

	r := &spec.Clusters{
		K8S:           ir,
		LoadBalancers: current.GetLoadBalancers(),
	}

	return r, events, nil
}

func syncWithRemoteRepo(nps []*spec.NodePool) error {
	for _, n := range nps {
		n := n.GetDynamicNodePool()
		if n == nil || n.Provider.Templates.Tag != nil {
			continue
		}

		if err := manifest.FetchCommitHash(n.Provider.Templates); err != nil {
			return err
		}
	}

	return nil
}

func transferTemplatesRepo(into, from []*spec.NodePool) {
	for _, d := range from {
		dn := d.GetDynamicNodePool()
		if dn == nil {
			continue
		}

		ci := slices.IndexFunc(into, func(c *spec.NodePool) bool { return c.Name == d.Name })
		if ci < 0 {
			continue
		}

		into[ci].GetDynamicNodePool().Provider.Templates = dn.Provider.Templates
	}
}

func nodepoolNames(nps []*spec.NodePool) map[string]struct{} {
	m := make(map[string]struct{})

	for _, c := range nps {
		m[c.Name] = struct{}{}
	}

	return m
}

// templatesUpdated check if at least 1 provider had their template repository updated.
func templatesUpdated(c *spec.Config) (bool, error) {
	for _, cluster := range c.Clusters {
		for _, n := range cluster.Current.GetK8S().GetClusterInfo().GetNodePools() {
			n := n.GetDynamicNodePool()
			if n == nil || n.Provider.Templates.Tag != nil {
				continue
			}

			updated, err := commitHashUpdated(n.Provider)
			if err != nil {
				return false, err
			}

			if updated {
				return true, nil
			}
		}

		for _, lb := range cluster.Current.GetLoadBalancers().GetClusters() {
			for _, n := range lb.ClusterInfo.NodePools {
				n := n.GetDynamicNodePool()
				if n == nil || n.Provider.Templates.Tag != nil {
					continue
				}

				updated, err := commitHashUpdated(n.Provider)
				if err != nil {
					return false, err
				}

				if updated {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func commitHashUpdated(p *spec.Provider) (bool, error) {
	t := proto.Clone(p.Templates).(*spec.TemplateRepository)
	if err := manifest.FetchCommitHash(t); err != nil {
		return false, err
	}

	return p.Templates.CommitHash != t.CommitHash, nil
}
