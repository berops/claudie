package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/internal/worker"
	"github.com/Berops/claudie/services/context-box/server/checksum"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/Berops/claudie/internal/healthcheck"
	"github.com/Berops/claudie/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedContextBoxServiceServer
}

const (
	defaultContextBoxPort = 50055
)

var (
	database ClaudieDB //database handle
)

// SaveConfigScheduler is a gRPC service: the function saves config to the DB after receiving it from Scheduler
func (*server) SaveConfigScheduler(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Info().Msg("CLIENT REQUEST: SaveConfigScheduler")
	config := req.GetConfig()
	err := utils.CheckLengthOfFutureDomain(config)
	if err != nil {
		return nil, fmt.Errorf("error while checking the length of future domain: %v", err)
	}

	// Save new config to the DB
	config.DsChecksum = config.MsChecksum
	config.SchedulerTTL = 0
	err = database.UpdateDs(config)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Error while updating dsChecksum and : %v", err),
		)
	}

	err = database.UpdateSchedulerTTL(config.Name, config.SchedulerTTL)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Error while update schedulerTTL: %v", err),
		)
	}

	return &pb.SaveConfigResponse{Config: config}, nil
}

// SaveConfigFrontEnd is a gRPC service: the function saves config to the DB after receiving it from Frontend
func (*server) SaveConfigFrontEnd(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Info().Msg("CLIENT REQUEST: SaveConfigFrontEnd")
	newConfig := req.GetConfig()
	newConfig.MsChecksum = checksum.CalculateChecksum(newConfig.Manifest)

	//check if any data already present for the newConfig
	oldConfig, err := database.GetConfig(newConfig.GetName(), pb.IdType_NAME)
	if err != nil {
		log.Info().Msgf("No existing doc with name: %v", newConfig.Name)
	} else {
		if string(oldConfig.MsChecksum) != string(newConfig.MsChecksum) {
			oldConfig.MsChecksum = newConfig.MsChecksum
			oldConfig.Manifest = newConfig.Manifest
			oldConfig.SchedulerTTL = 0
			oldConfig.BuilderTTL = 0
		}
		newConfig = oldConfig
	}

	// save config to DB
	err = database.SaveConfig(newConfig)
	if err != nil {
		return nil, fmt.Errorf("error while saving config %s in db : %v", newConfig.Name, err)
	}

	return &pb.SaveConfigResponse{Config: newConfig}, nil
}

// SaveConfigBuilder is a gRPC service: the function saves config to the DB after receiving it from Builder
func (*server) SaveConfigBuilder(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Info().Msg("CLIENT REQUEST: SaveConfigBuilder")
	config := req.GetConfig()

	// Get config with the same ID from the DB
	databaseConfig, err := database.GetConfig(config.GetName(), pb.IdType_NAME)
	if err != nil {
		return nil, err
	}
	// check if the DsChecksum from DB and config object are nil.
	// If they are nill , we want to delete the document from the DB
	if config.DsChecksum == nil && databaseConfig.DsChecksum == nil && config.ErrorMessage != "" {
		err = database.DeleteConfig(config.Id, pb.IdType_HASH)
		if err != nil {
			return nil, err
		}
		return &pb.SaveConfigResponse{Config: config}, nil
	}
	// Save new config to the DB
	config.CsChecksum = config.DsChecksum
	config.BuilderTTL = 0
	err = database.UpdateCs(config)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Error while updating current state: %v", err),
		)
	}

	err = database.UpdateBuilderTTL(config.Name, config.BuilderTTL)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Error while update builderTTL: %v", err),
		)
	}
	return &pb.SaveConfigResponse{Config: config}, nil
}

// GetConfigById is a gRPC service: function returns one config from the DB based on the requested index/name
func (*server) GetConfigFromDB(ctx context.Context, req *pb.GetConfigFromDBRequest) (*pb.GetConfigFromDBResponse, error) {
	log.Info().Msg("CLIENT REQUEST: GetConfigFromDB")
	config, err := database.GetConfig(req.Id, req.Type)
	if err != nil {
		log.Error().Msgf("Error while getting a config from database : %v", err)
		return nil, err
	}
	return &pb.GetConfigFromDBResponse{Config: config}, nil
}

// GetConfigScheduler is a gRPC service: function returns oldest config from the queueScheduler
func (*server) GetConfigScheduler(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Info().Msg("GetConfigScheduler request")
	configInfo := queueScheduler.Dequeue()
	if configInfo != nil {
		config, err := database.GetConfig(configInfo.GetName(), pb.IdType_NAME)
		if err != nil {
			return nil, err
		}
		return &pb.GetConfigResponse{Config: config}, nil
	}
	return nil, fmt.Errorf("empty Scheduler queue")
}

// GetConfigBuilder is a gRPC service: function returns oldest config from the queueBuilder
func (*server) GetConfigBuilder(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Info().Msg("GetConfigBuilder request")
	configInfo := queueBuilder.Dequeue()
	if configInfo != nil {
		config, err := database.GetConfig(configInfo.GetName(), pb.IdType_NAME)
		if err != nil {
			return nil, err
		}
		return &pb.GetConfigResponse{Config: config}, nil
	}
	return nil, fmt.Errorf("empty Builder queue")
}

// GetAllConfigs is a gRPC service: function returns all configs from the DB
func (*server) GetAllConfigs(ctx context.Context, req *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
	log.Info().Msg("CLIENT REQUEST: GetAllConfigs")
	configs, err := database.GetAllConfigs()
	if err != nil {
		return nil, fmt.Errorf("error getting all configs : %v", err)
	}
	return &pb.GetAllConfigsResponse{Configs: configs}, nil
}

// DeleteConfig is a gRPC service: function deletes one specified config from the DB and returns it's ID
func (*server) DeleteConfig(ctx context.Context, req *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	log.Info().Msg("CLIENT REQUEST: DeleteConfig")
	err := database.UpdateMsToNull(req.Id)
	if err != nil {
		return nil, err
	}

	return &pb.DeleteConfigResponse{Id: req.GetId()}, nil
}

func main() {
	// initialize logger & db
	utils.InitLog("context-box")
	var err error
	database, err = initDatabase()
	if err != nil {
		log.Fatal().Msgf("Failed to connect to the database, aborting... : %v", err)
	}
	defer func() {
		err := database.Disconnect()
		if err != nil {
			log.Fatal().Msgf("Error while closing the connection to the database : %v", err)
		}
	}()

	// Start ContextBox Service
	contextboxPort := utils.GetenvOr("CONTEXT_BOX_PORT", fmt.Sprint(defaultContextBoxPort))
	contextBoxAddr := net.JoinHostPort("0.0.0.0", contextboxPort)
	lis, err := net.Listen("tcp", contextBoxAddr)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on contextbox addr %s : %v", contextBoxAddr, err)
	}
	log.Info().Msgf("ContextBox service is listening on: %s", contextBoxAddr)

	// start the gRPC server
	s := grpc.NewServer()
	pb.RegisterContextBoxServiceServer(s, &server{})

	// Add health service to gRPC
	healthService := healthcheck.NewServerHealthChecker(contextboxPort, "CONTEXT_BOX_PORT", nil)
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, ctx := errgroup.WithContext(context.Background())
	w := worker.NewWorker(ctx, 10*time.Second, configChecker, worker.ErrorLogger)
	// listen for system interrupts to gracefully shut down
	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		<-ch
		signal.Stop(ch)
		s.GracefulStop()
		return errors.New("ContextBox interrupt signal")
	})
	// server goroutine
	g.Go(func() error {
		// s.Serve() will create a service goroutine for each connection
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("ContextBox failed to serve: %v", err)
		}
		return nil
	})
	//config checker go routine
	g.Go(func() error {
		w.Run()
		return nil
	})
	log.Info().Msgf("Stopping Context-Box: %v", g.Wait())
}
