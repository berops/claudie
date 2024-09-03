package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/server/internal/store"
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
			return nil, status.Errorf(codes.Internal, "failed to check existance for config %q: %v", request.Name, err)
		}
		return nil, status.Errorf(codes.NotFound, "no config with name %q found", request.Name)
	}

	if dbConfig.Version != request.Version {
		return nil, status.Errorf(codes.Aborted, "config %q with version %v was not found", request.Name, request.Version)
	}

	if _, ok := dbConfig.Clusters[request.Cluster]; !ok {
		return nil, status.Errorf(codes.NotFound, "failed to find cluster %q within config %q", request.Cluster, request.Name)
	}

	// We need initiate a build process due to the possible change
	// of the nodepool nodes.
	grpc, err := store.ConvertToGRPC(dbConfig)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert database representation for config %q to grpc: %v", request.Name, err)
	}

	cluster := grpc.Clusters[request.Cluster]

	if dbConfig.Manifest.State == manifest.Scheduled.String() && len(cluster.Events.Events) != 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "can't update nodepool of a cluster on which changes are being currently done")
	}

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

	ci := slices.IndexFunc(cnp, func(p *spec.NodePool) bool { return p.Name == request.Nodepool.Name })
	di := slices.IndexFunc(dnp, func(p *spec.NodePool) bool { return p.Name == request.Nodepool.Name })

	diffResult := nodeDiff(cnp[ci], request.Nodepool)

	updateNodePool(
		diffResult,
		fmt.Sprintf("%s-%s", utils.GetClusterID(cluster.Current.K8S.ClusterInfo), request.Nodepool.Name),
		dnp[di],
	)

	grpc.Manifest.State = spec.Manifest_Scheduled

	cluster.Events = &spec.Events{
		Events:     autoscaledEvents(diffResult, cluster.Desired),
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
}

func nodeDiff(current, desired *spec.NodePool) nodeDiffResult {
	result := nodeDiffResult{
		nodepool: desired.Name,
		oldCount: int(current.GetDynamicNodePool().Count),
		newCount: int(desired.GetDynamicNodePool().Count),
	}

	for _, n := range current.Nodes {
		deleted := !slices.ContainsFunc(desired.Nodes, func(n2 *spec.Node) bool { return n2.Name == n.Name })
		if deleted {
			result.deleted = append(result.deleted, n)
		}
	}

	for _, n := range current.Nodes {
		reused := !slices.ContainsFunc(result.deleted, func(n2 *spec.Node) bool { return n2.Name == n.Name })
		if reused {
			result.reused = append(result.reused, n)
		}
	}

	for _, n := range desired.Nodes {
		added := !slices.ContainsFunc(current.Nodes, func(n2 *spec.Node) bool { return n.Name == n2.Name })
		if added {
			result.added = append(result.added, n)
		}
	}

	return result
}

func updateNodePool(diff nodeDiffResult, nodePoolID string, desired *spec.NodePool) {
	usedNames := make(map[string]struct{})
	var newNodes []*spec.Node

	for _, n := range diff.deleted {
		log.Debug().Str("nodepool", nodePoolID).Msgf("node %q deleted from desired nodepool %q after autoscaler update", n.Name, desired.Name)
		usedNames[n.Name] = struct{}{}
	}

	for _, n := range diff.reused {
		log.Debug().Str("nodepool", nodePoolID).Msgf("node %q resued to desired nodepool %q after autoscaler update", n.Name, desired.Name)
		newNodes = append(newNodes, n)
		usedNames[n.Name] = struct{}{}
	}

	for _, n := range diff.added {
		log.Debug().Str("nodepool", nodePoolID).Msgf("node %q added to desired nodepool %q after autoscaler update", n.Name, desired.Name)
		newNodes = append(newNodes, n)
		usedNames[n.Name] = struct{}{}
	}

	for len(newNodes) < diff.newCount {
		c := len(newNodes)
		name := uniqueNodeName(nodePoolID, usedNames)
		usedNames[name] = struct{}{}
		newNodes = append(newNodes, &spec.Node{Name: name})
		log.Debug().Str("nodepool", nodePoolID).Msgf("node %q added to desired nodepool %q after autoscaler update (%v < %v)", name, desired.Name, c, diff.newCount)
	}

	desired.GetDynamicNodePool().Count = int32(diff.newCount)
	desired.Nodes = newNodes
}

func autoscaledEvents(diff nodeDiffResult, desired *spec.Clusters) []*spec.TaskEvent {
	var events []*spec.TaskEvent

	if diff.oldCount < diff.newCount || len(diff.added) > 0 {
		events = append(events, &spec.TaskEvent{
			Id:          uuid.New().String(),
			Timestamp:   timestamppb.New(time.Now().UTC()),
			Event:       spec.Event_UPDATE,
			Description: "autoscaler: adding nodes to k8s cluster",
			Task: &spec.Task{
				UpdateState: &spec.UpdateState{
					K8S: desired.K8S, // changes to the desired nodepool should have been done at this point.
					Lbs: desired.GetLoadBalancers(),
				},
			},
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
		})
	}

	return events
}
