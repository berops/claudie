package grpc

import (
	"context"

	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/context-box/server/domain/usecases"
)

type ContextBoxGrpcService struct {
	pb.UnimplementedContextBoxServiceServer

	usecases *usecases.Usecases
}

// Saves config to MongoDB after receiving it from the frontend microservice
func (c *ContextBoxGrpcService) SaveConfigFrontend(ctx context.Context, request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	return c.usecases.SaveConfigFrontend(request)
}

// SaveWorkflowState updates the workflow for a single cluster
func (c *ContextBoxGrpcService) SaveWorkflowState(ctx context.Context, request *pb.SaveWorkflowStateRequest) (*pb.SaveWorkflowStateResponse, error) {
	return c.usecases.SaveWorkflowState(request)
}

// SaveConfigScheduler is a gRPC servie: the function saves config to the DB after receiving it from Scheduler
func (c *ContextBoxGrpcService) SaveConfigScheduler(ctx context.Context, request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	return c.usecases.SaveConfigScheduler(request)
}

// SaveConfigBuilder is a gRPC service: the function saves config to the DB after receiving it from Builder
func (c *ContextBoxGrpcService) SaveConfigBuilder(ctx context.Context, request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	return c.usecases.SaveConfigBuilder(request)
}

// GetConfigById is a gRPC service: function returns one config from the DB based on the requested index/name
func (c *ContextBoxGrpcService) GetConfigFromDB(ctx context.Context, request *pb.GetConfigFromDBRequest) (*pb.GetConfigFromDBResponse, error) {
	return c.usecases.GetConfigFromDB(request)
}

// GetConfigScheduler is a gRPC service: function returns oldest config from the queueScheduler
func (c *ContextBoxGrpcService) GetConfigScheduler(ctx context.Context, request *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	return c.usecases.GetConfigScheduler(request)
}

// GetConfigBuilder is a gRPC service: function returns oldest config from the queueBuilder
func (c *ContextBoxGrpcService) GetConfigBuilder(ctx context.Context, request *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	return c.usecases.GetConfigBuilder(request)
}

// GetAllConfigs is a gRPC service: function returns all configs from the DB
func (c *ContextBoxGrpcService) GetAllConfigs(ctx context.Context, request *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
	return c.usecases.GetAllConfigs(request)
}

// DeleteConfig sets the manifest to nil so that the iteration workflow for this
// config destroys the previous build infrastructure.
func (c *ContextBoxGrpcService) DeleteConfig(ctx context.Context, request *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	return c.usecases.DeleteConfig(request)
}

// DeleteConfigFromDB removes the config from the request from the mongoDB c.usecases.MongoDB.
func (c *ContextBoxGrpcService) DeleteConfigFromDB(ctx context.Context, request *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	return c.usecases.DeleteConfigFromDB(request)
}

// UpdateNodepool updates the Nodepool struct in the database, which also initiates build. This function might return an error if the updation is
// not allowed at this time (i.e.when config is being build).
func (c *ContextBoxGrpcService) UpdateNodepool(ctx context.Context, request *pb.UpdateNodepoolRequest) (*pb.UpdateNodepoolResponse, error) {
	return c.usecases.UpdateNodepool(request)
}
