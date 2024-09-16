package service

import (
	"context"
	"errors"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (g *GRPC) UpdateCurrentState(ctx context.Context, request *pb.UpdateCurrentStateRequest) (*pb.UpdateCurrentStateResponse, error) {
	if request.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of config")
	}
	if request.Cluster == "" {
		return nil, status.Errorf(codes.InvalidArgument, "missing name of cluster")
	}

	log.Debug().Msgf("Updating current state for Config: %q Cluster: %q Version: %v", request.Name, request.Cluster, request.Version)

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

	grpc, err := store.ConvertToGRPC(dbConfig)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert database representation for config %q to grpc: %v", request.Name, err)
	}

	cluster, exists := grpc.Clusters[request.Cluster]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "failed to find cluster %q within config %q", request.Cluster, request.Name)
	}

	cluster.Current = request.State

	if cluster.Current != nil && cluster.Desired != nil { // on update.
		log.Debug().Str("cluster", utils.GetClusterID(cluster.Current.K8S.ClusterInfo)).Msgf("transferring state from newly supplied current state into desired state")

		if err := transferExistingK8sState(cluster.Current.K8S, cluster.Desired.K8S); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to trasnsfer updated current state to desired state for cluster %q config %q: %v", request.Cluster, request.Name, err)
		}

		if err := transferExistingLBState(cluster.Current.LoadBalancers, cluster.Desired.LoadBalancers); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to trasnsfer updated current state to desired state for cluster %q config %q: %v", request.Cluster, request.Name, err)
		}
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

	return &pb.UpdateCurrentStateResponse{Name: request.Name, Version: db.Version}, nil
}
