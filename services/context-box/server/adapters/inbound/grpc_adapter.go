package inboundAdapters

import (
	"context"
	"fmt"
	"net"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/context-box/server/domain/usecases"
)

const (
	defaultContextBoxPort = 50055
)

type GrpcAdapter struct {
	tcpListener       net.Listener
	server            *grpc.Server
	healthCheckServer *health.Server
}

func (g *GrpcAdapter) Init(usecases *usecases.Usecases) {

	port := utils.GetenvOr("CONTEXT_BOX_PORT", fmt.Sprint(defaultContextBoxPort))
	listeningAddress := net.JoinHostPort("0.0.0.0", port)

	tcpListener, err := net.Listen("tcp", listeningAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to start Grpc server for context-box microservice at %s: %v", listeningAddress, err)
	}
	g.tcpListener = tcpListener

	log.Info().Msgf("context-box microservice bound to %s", listeningAddress)

	g.server = grpc.NewServer()
	pb.RegisterContextBoxServiceServer(g.server, &ContextBoxGrpcServiceImplementation{usecases: usecases})

	// Add health-check service to gRPC
	g.healthCheckServer = health.NewServer()
	// Context-box does not have any custom health check functions, thus always serving.
	g.healthCheckServer.SetServingStatus("context-box-liveness", grpc_health_v1.HealthCheckResponse_SERVING)
	g.healthCheckServer.SetServingStatus("context-box-readiness", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(g.server, g.healthCheckServer)
}

func (g *GrpcAdapter) Serve() error {
	// g.server.Serve( ) will create a service goroutine for each connection
	if err := g.server.Serve(g.tcpListener); err != nil {
		return fmt.Errorf("Context-box microservice grpc server failed to serve: %v", err)
	}

	log.Info().Msgf("Finished listening for incomig gRPC connections")
	return nil
}

func (g *GrpcAdapter) Stop() {
	g.server.GracefulStop()
	g.healthCheckServer.Shutdown()
}

func NewGrpcAdapter() *GrpcAdapter {
	return &GrpcAdapter{}
}

//

type ContextBoxGrpcServiceImplementation struct {
	pb.UnimplementedContextBoxServiceServer

	usecases *usecases.Usecases
}

// Saves config to MongoDB after receiving it from the frontend microservice
func (c *ContextBoxGrpcServiceImplementation) SaveConfigFrontend(ctx context.Context, request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	return c.usecases.SaveConfigFrontend(request)
}

// SaveWorkflowState updates the workflow for a single cluster
func (c *ContextBoxGrpcServiceImplementation) SaveWorkflowState(ctx context.Context, request *pb.SaveWorkflowStateRequest) (*pb.SaveWorkflowStateResponse, error) {
	return c.usecases.SaveWorkflowState(request)
}

// SaveConfigScheduler is a gRPC servie: the function saves config to the DB after receiving it from Scheduler
func (c *ContextBoxGrpcServiceImplementation) SaveConfigScheduler(ctx context.Context, request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	return c.usecases.SaveConfigScheduler(request)
}

// SaveConfigBuilder is a gRPC service: the function saves config to the DB after receiving it from Builder
func (c *ContextBoxGrpcServiceImplementation) SaveConfigBuilder(ctx context.Context, request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	return c.usecases.SaveConfigBuilder(request)
}

// GetConfigById is a gRPC service: function returns one config from the DB based on the requested index/name
func (c *ContextBoxGrpcServiceImplementation) GetConfigFromDB(ctx context.Context, request *pb.GetConfigFromDBRequest) (*pb.GetConfigFromDBResponse, error) {
	return c.usecases.GetConfigFromDB(request)
}

// GetConfigScheduler is a gRPC service: function returns oldest config from the queueScheduler
func (c *ContextBoxGrpcServiceImplementation) GetConfigScheduler(ctx context.Context, request *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	return c.usecases.GetConfigScheduler(request)
}

// GetConfigBuilder is a gRPC service: function returns oldest config from the queueBuilder
func (c *ContextBoxGrpcServiceImplementation) GetConfigBuilder(ctx context.Context, request *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	return c.usecases.GetConfigBuilder(request)
}

// GetAllConfigs is a gRPC service: function returns all configs from the DB
func (c *ContextBoxGrpcServiceImplementation) GetAllConfigs(ctx context.Context, request *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
	return c.usecases.GetAllConfigs(request)
}

// DeleteConfig sets the manifest to nil so that the iteration workflow for this
// config destroys the previous build infrastructure.
func (c *ContextBoxGrpcServiceImplementation) DeleteConfig(ctx context.Context, request *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	return c.usecases.DeleteConfig(request)
}

// DeleteConfigFromDB removes the config from the request from the mongoDB c.usecases.MongoDB.
func (c *ContextBoxGrpcServiceImplementation) DeleteConfigFromDB(ctx context.Context, request *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	return c.usecases.DeleteConfigFromDB(request)
}
