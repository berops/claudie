package service

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// rollingUpdateLBs does the same thing as rolling update for k8s.
func rollingUpdateLBs(current, desired *spec.Clusters) (*spec.Clusters, []*spec.TaskEvent, error) {
	for i := range current.GetLoadBalancers().GetClusters() {
		ir, events, err := rollingUpdateLB(current, desired, i)
		if err != nil {
			return nil, nil, err
		}
		if len(events) > 0 {
			return ir, events, nil
		}
	}

	return nil, nil, nil
}

func rollingUpdateLB(current, desired *spec.Clusters, position int) (*spec.Clusters, []*spec.TaskEvent, error) {
	var (
		events            []*spec.TaskEvent
		mapping           = make(map[int]*spec.NodePool)
		usedNodePoolNames = make(map[string]struct{})
		ir                = proto.Clone(current.LoadBalancers).(*spec.LoadBalancers)
		rollingUpdates    = proto.Clone(current.LoadBalancers).(*spec.LoadBalancers)
		lbID              = ir.Clusters[position].ClusterInfo.Id()
	)

	indexDesired := slices.IndexFunc(desired.GetLoadBalancers().GetClusters(), func(d *spec.LBcluster) bool {
		return d.ClusterInfo.Id() == lbID
	})

	// names are unique within a single cluster.
	maps.Insert(
		usedNodePoolNames,
		maps.All(nodepoolNames(current.LoadBalancers.Clusters[position].ClusterInfo.NodePools)),
	)
	if indexDesired >= 0 {
		maps.Insert(
			usedNodePoolNames,
			maps.All(nodepoolNames(desired.LoadBalancers.Clusters[indexDesired].ClusterInfo.NodePools)),
		)
	}

	if err := syncWithRemoteRepo(ir.Clusters[position].ClusterInfo.NodePools); err != nil {
		return nil, nil, err
	}

	if indexDesired >= 0 {
		transferTemplatesRepo(
			ir.Clusters[position].ClusterInfo.NodePools,
			desired.LoadBalancers.Clusters[indexDesired].ClusterInfo.NodePools,
		)
	}

	for di, d := range ir.Clusters[position].ClusterInfo.NodePools {
		desiredPool := d.GetDynamicNodePool()
		if desiredPool == nil {
			continue
		}

		ci := slices.IndexFunc(rollingUpdates.Clusters[position].ClusterInfo.NodePools, func(c *spec.NodePool) bool {
			return c.Name == d.Name
		})
		currentPool := rollingUpdates.Clusters[position].ClusterInfo.NodePools[ci]
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
		nodepoolID := fmt.Sprintf("%s-%s", lbID, updated.Name)
		generateMissingDynamicNodes(nodepoolID, nodeNames, updated)

		rollback := proto.Clone(rollingUpdates).(*spec.LoadBalancers)

		rollingUpdates.Clusters[position].ClusterInfo.NodePools = append(rollingUpdates.Clusters[position].ClusterInfo.NodePools, updated)
		addNodePool := proto.Clone(rollingUpdates).(*spec.LoadBalancers)

		rollingUpdates.Clusters[position].ClusterInfo.NodePools = slices.Delete(rollingUpdates.Clusters[position].ClusterInfo.NodePools, ci, ci+1)
		delNodePool := proto.Clone(rollingUpdates).(*spec.LoadBalancers)

		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: fmt.Sprintf("rolling update loadbalancers: replacing %s with %s", currentPool.Name, updated.Name),
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: current.K8S, // keep current k8s
					Lbs: addNodePool,
				},
			},
			OnError: &spec.Retry{Do: &spec.Retry_Rollback_{Rollback: &spec.Retry_Rollback{
				Tasks: []*spec.TaskEvent{
					{
						Id:          uuid.New().String(),
						Timestamp:   timestamppb.New(time.Now().UTC()),
						Event:       spec.Event_UPDATE,
						Description: fmt.Sprintf("rollback lbs: deleting infrastructure of deleted nodepool %s", updated.Name),
						Task: &spec.Task{
							UpdateState: &spec.UpdateState{
								K8S: current.K8S, // keep current k8s
								Lbs: rollback,
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
			Str("cluster", lbID).
			Msgf("created event %q with Rollback on error %q with repeat on rollback failure",
				events[len(events)-1].Description,
				events[len(events)-1].OnError.Do.(*spec.Retry_Rollback_).Rollback.Tasks[0].Description,
			)

		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: fmt.Sprintf("rolling update lbs: deleting infrastructure of deleted nodes from nodepool %s", currentPool.Name),
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: current.K8S, // keep current k8s
					Lbs: delNodePool,
				},
			},
			OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
				Kind: spec.Retry_Repeat_ENDLESS,
			}}},
		})

		log.Debug().
			Str("cluster", lbID).
			Msgf("created event %q with Repeat on error", events[len(events)-1].Description)
	}

	for di, updated := range mapping {
		ir.Clusters[position].ClusterInfo.NodePools[di] = updated
	}

	r := &spec.Clusters{
		K8S:           current.K8S,
		LoadBalancers: ir,
	}

	return r, events, nil
}
