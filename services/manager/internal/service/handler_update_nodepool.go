package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (g *GRPC) UpdateNodePool(ctx context.Context, request *pb.UpdateNodePoolRequest) (*pb.UpdateNodePoolResponse, error) {
	if request.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of config")
	}
	if request.Cluster == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of cluster")
	}
	if request.Nodepool == nil || request.Nodepool.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing nodepool to update")
	}

	log.Debug().Msgf("Updating NodePool for Config: %q Cluster: %q Version: %v", request.Name, request.Cluster, request.Version)

	dbConfig, err := g.Store.GetConfig(ctx, request.Name)
	if err != nil {
		if !errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Internal, "failed to check existence for config %q: %v", request.Name, err)
		}
		return nil, status.Errorf(codes.NotFound, "no config with name %q found", request.Name)
	}

	if dbConfig.Version != request.Version {
		return nil, status.Errorf(codes.Aborted, "config %q with version %v was not found", request.Name, request.Version)
	}

	if _, ok := dbConfig.Clusters[request.Cluster]; !ok {
		return nil, status.Errorf(codes.NotFound, "failed to find cluster %q within config %q", request.Cluster, request.Name)
	}

	finished := bytes.Equal(dbConfig.Manifest.LastAppliedChecksum, dbConfig.Manifest.Checksum)
	finished = finished && (slices.Contains(
		[]string{manifest.Done.String(), manifest.Error.String()},
		dbConfig.Manifest.State,
	))
	if !finished {
		return nil, status.Errorf(codes.FailedPrecondition, "can't update nodepool %q cluster %q from configuration on which changes are currently ongoing.", request.Cluster, request.Name)
	}

	grpc, err := store.ConvertToGRPC(dbConfig)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert database representation for config %q to grpc: %v", request.Name, err)
	}

	cluster := grpc.Clusters[request.Cluster]

	cnp := cluster.GetCurrent().GetK8S().GetClusterInfo().GetNodePools()
	dnp := cluster.GetDesired().GetK8S().GetClusterInfo().GetNodePools()

	found := slices.ContainsFunc(cnp, func(p *spec.NodePool) bool { return p.Name == request.Nodepool.Name })
	found = found && slices.ContainsFunc(dnp, func(p *spec.NodePool) bool { return p.Name == request.Nodepool.Name })
	if !found {
		return nil, status.Errorf(codes.NotFound, "nodepool %q not found in current and desired state (might be present in one of them but not both)", request.Nodepool.Name)
	}

	// every change moves the manifest state into pending.
	ok, err := manifest.ValidStateTransitionString(dbConfig.Manifest.State, manifest.Pending)
	if err != nil || !ok {
		return nil, status.Errorf(codes.FailedPrecondition, "can't move manifest from state %q to state %q", dbConfig.Manifest.State, manifest.Scheduled.String())
	}

	// since we want to skip creating the desired state anew move immediately to scheduled.
	if ok = manifest.ValidStateTransition(manifest.Pending, manifest.Scheduled); !ok {
		return nil, status.Errorf(codes.FailedPrecondition, "can't move manifest from state %q to state %q", dbConfig.Manifest.State, manifest.Scheduled.String())
	}

	grpc.Manifest.State = spec.Manifest_Scheduled

	ci := slices.IndexFunc(cnp, func(p *spec.NodePool) bool { return p.Name == request.Nodepool.Name })
	di := slices.IndexFunc(dnp, func(p *spec.NodePool) bool { return p.Name == request.Nodepool.Name })

	diffResult := nodeDiff(
		fmt.Sprintf("%s-%s", cluster.Current.K8S.ClusterInfo.Id(), request.Nodepool.Name),
		cnp[ci],
		request.Nodepool,
	)

	// prepare desired nodepool with new nodes.
	dnp[di].GetDynamicNodePool().Count = int32(diffResult.newCount)
	dnp[di].Nodes = append(append([]*spec.Node(nil), diffResult.reused...), diffResult.added...)

	cluster.Events = &spec.Events{
		Events:     autoscaledEvents(diffResult, cluster.Current, cluster.Desired),
		Autoscaled: true,
	}

	cluster.State = &spec.Workflow{
		Stage:  spec.Workflow_NONE,
		Status: spec.Workflow_DONE,
	}

	db, err := store.ConvertFromGRPC(grpc)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert config %q from grpc representation to database representation: %v", request.Name, err)
	}

	// change the last applied checksum to a dummy value,
	// so that the config will be rescheduled again to trigger
	// the OnError handler if needed.
	db.Manifest.LastAppliedChecksum = db.Manifest.LastAppliedChecksum[:len(db.Manifest.LastAppliedChecksum)-1]

	if err := g.Store.UpdateConfig(ctx, db); err != nil {
		if errors.Is(err, store.ErrNotFoundOrDirty) {
			return nil, status.Errorf(codes.Aborted, "couldn't update config %q with version %v, dirty write", request.Name, request.Version)
		}
		return nil, status.Errorf(codes.Internal, "failed to update current state for cluster: %q config: %q", request.Cluster, request.Name)
	}

	return &pb.UpdateNodePoolResponse{Name: request.Name, Version: db.Version}, nil
}

type nodeDiffResult struct {
	nodepool               string
	deleted, added, reused []*spec.Node
	oldCount, newCount     int
	endpointDeleted        bool
}

func nodeDiff(nodepoolId string, current, desired *spec.NodePool) nodeDiffResult {
	usedNames := make(map[string]struct{})
	result := nodeDiffResult{
		nodepool: desired.Name,
		oldCount: int(current.GetDynamicNodePool().Count),
		newCount: int(desired.GetDynamicNodePool().Count),
	}

	for _, n := range current.Nodes {
		canReuse := slices.ContainsFunc(desired.Nodes, func(n2 *spec.Node) bool { return n2.Name == n.Name })
		canReuse = canReuse && len(result.reused) < result.newCount
		if canReuse {
			log.Debug().
				Str("nodepool", nodepoolId).
				Msgf("node %q resued to desired nodepool %q after autoscaler update", n.Name, desired.Name)

			result.reused = append(result.reused, n)
			usedNames[n.Name] = struct{}{}
		} else {
			log.Debug().
				Str("nodepool", nodepoolId).
				Msgf("node %q deleted from desired nodepool %q after autoscaler update", n.Name, desired.Name)

			result.deleted = append(result.deleted, n)
			usedNames[n.Name] = struct{}{}
			if n.NodeType == spec.NodeType_apiEndpoint {
				result.endpointDeleted = true
			}
		}
	}

	for _, n := range desired.Nodes {
		canAdd := !slices.ContainsFunc(current.Nodes, func(n2 *spec.Node) bool { return n.Name == n2.Name })
		canAdd = canAdd && (len(result.reused)+len(result.added) < result.newCount)
		if canAdd {
			log.Debug().
				Str("nodepool", nodepoolId).
				Msgf("node %q added to desired nodepool %q after autoscaler update", n.Name, desired.Name)

			result.added = append(result.added, n)
			usedNames[n.Name] = struct{}{}
		}
	}

	for range max(0, result.newCount-(len(result.reused)+len(result.added))) {
		name := uniqueNodeName(nodepoolId, usedNames)
		usedNames[name] = struct{}{}
		result.added = append(result.added, &spec.Node{Name: name})
		log.Debug().
			Str("nodepool", nodepoolId).
			Msgf("node %q generated to desired nodepool %q after autoscaler update", name, desired.Name)
	}

	return result
}

func autoscaledEvents(diff nodeDiffResult, current, desired *spec.Clusters) []*spec.TaskEvent {
	var events []*spec.TaskEvent

	if diff.oldCount < diff.newCount || len(diff.added) > 0 {
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: "autoscaler: adding nodes to k8s cluster",
			Task: &spec.Task{UpdateState: &spec.UpdateState{
				K8S: desired.K8S, // changes to the desired nodepool should have been done at this point.
				Lbs: desired.GetLoadBalancers(),
			}},
			OnError: &spec.Retry{Do: &spec.Retry_Rollback_{
				Rollback: &spec.Retry_Rollback{
					Tasks: []*spec.TaskEvent{
						{
							Id:          uuid.New().String(),
							Timestamp:   timestamppb.New(time.Now().UTC()),
							Event:       spec.Event_DELETE,
							Description: fmt.Sprintf("autoscaler rollback: deleting nodes from nodepool %s", diff.nodepool),
							Task: &spec.Task{DeleteState: &spec.DeleteState{
								Nodepools: map[string]*spec.DeletedNodes{
									diff.nodepool: {
										Nodes: func() []string {
											var result []string
											for _, n := range diff.added {
												result = append(result, n.Name)
											}
											return result
										}(),
									},
								},
							}},
							OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
								Kind:        spec.Retry_Repeat_EXPONENTIAL,
								CurrentTick: 1,
								StopAfter:   uint32(25 * time.Minute / Tick),
							}}},
						},
						{
							Id:          uuid.New().String(),
							Timestamp:   timestamppb.New(time.Now().UTC()),
							Event:       spec.Event_UPDATE,
							Description: fmt.Sprintf("autoscaler rollback: deleting nodes from nodepool %s", diff.nodepool),
							Task: &spec.Task{UpdateState: &spec.UpdateState{
								K8S: current.K8S,
								Lbs: current.LoadBalancers,
							}},
							OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
								Kind:        spec.Retry_Repeat_EXPONENTIAL,
								CurrentTick: 1,
								StopAfter:   uint32(25 * time.Minute / Tick),
							}}},
						},
					},
				},
			}},
		})
	}

	if diff.endpointDeleted {
		nodePool, node := newAPIEndpointNodeCandidate(desired.K8S.ClusterInfo.NodePools)
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: "autoscaler: moving endpoint from old control plane node to a new control plane node",
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{Endpoint: &spec.UpdateState_Endpoint{
					Nodepool: nodePool,
					Node:     node,
				}},
			},
			OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
				Kind: spec.Retry_Repeat_ENDLESS,
			}}},
		})
	}

	if len(diff.deleted) > 0 {
		dn := map[string]*spec.DeletedNodes{diff.nodepool: new(spec.DeletedNodes)}
		for _, v := range diff.deleted {
			dn[diff.nodepool].Nodes = append(dn[diff.nodepool].Nodes, v.Name)
		}
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_DELETE,
			Description: "autoscaler: deleting nodes from k8s cluster",
			Task:        &spec.Task{DeleteState: &spec.DeleteState{Nodepools: dn}},
			OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
				Kind:        spec.Retry_Repeat_EXPONENTIAL,
				CurrentTick: 1,
				StopAfter:   uint32(25 * time.Minute / Tick),
			}}},
		})
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: "autoscaler: deleting infrastructure of deleted k8s nodes",
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: desired.K8S,
					Lbs: desired.GetLoadBalancers(),
				},
			},
			OnError: &spec.Retry{Do: &spec.Retry_Repeat_{Repeat: &spec.Retry_Repeat{
				Kind:        spec.Retry_Repeat_EXPONENTIAL,
				CurrentTick: 1,
				StopAfter:   uint32(25 * time.Minute / Tick),
			}}},
		})
	}

	return events
}
