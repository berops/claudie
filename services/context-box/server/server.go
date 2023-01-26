package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Berops/claudie/internal/envs"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/internal/worker"
	"github.com/Berops/claudie/services/context-box/server/checksum"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/Berops/claudie/internal/healthcheck"
	"github.com/Berops/claudie/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
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
	config := req.GetConfig()
	log.Info().Msgf("CLIENT REQUEST: SaveConfigScheduler for %s", config.Name)
	// Save new config to the DB
	config.DsChecksum = config.MsChecksum
	config.SchedulerTTL = 0
	err := database.UpdateDs(config)
	if err != nil {
		return nil, fmt.Errorf("error while updating dsChecksum for %s : %w", config.Name, err)
	}

	err = database.UpdateSchedulerTTL(config.Name, config.SchedulerTTL)
	if err != nil {
		return nil, fmt.Errorf("error while updating schedulerTTL for %s : %w", config.Name, err)
	}

	return &pb.SaveConfigResponse{Config: config}, nil
}

// SaveConfigFrontEnd is a gRPC service: the function saves config to the DB after receiving it from Frontend
func (*server) SaveConfigFrontEnd(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	newConfig := req.GetConfig()
	log.Info().Msgf("CLIENT REQUEST: SaveConfigFrontEnd for %s", newConfig.Name)
	newConfig.MsChecksum = checksum.CalculateChecksum(newConfig.Manifest)

	//check if any data already present for the newConfig
	oldConfig, err := database.GetConfig(newConfig.GetName(), pb.IdType_NAME)
	if err == nil {
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
		return nil, fmt.Errorf("error while saving config %s in db : %w", newConfig.Name, err)
	}

	return &pb.SaveConfigResponse{Config: newConfig}, nil
}

// SaveConfigBuilder is a gRPC service: the function saves config to the DB after receiving it from Builder
func (*server) SaveConfigBuilder(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	config := req.GetConfig()
	log.Info().Msgf("CLIENT REQUEST: SaveConfigBuilder for %s", config.Name)

	// Save new config to the DB, update csState as dsState
	config.CsChecksum = config.DsChecksum
	config.BuilderTTL = 0
	// In Builder, the desired state is also updated i.e. in terraformer (node IPs, etc) thus
	// we need to update it in database,
	// however, if deletion has been triggered, the desired state should be nil
	if dbConf, err := database.GetConfig(config.Id, pb.IdType_HASH); err != nil {
		log.Warn().Msgf("Got error while checking the desired state in the database : %v", err)
	} else {
		if dbConf.DesiredState != nil {
			if err := database.UpdateDs(config); err != nil {
				return nil, fmt.Errorf("Error while updating desired state: %v", err)
			}
		}
	}

	// Update the current state so its equal to the desired state
	if err := database.UpdateCs(config); err != nil {
		return nil, fmt.Errorf("error while updating csChecksum for %s : %w", config.Name, err)
	}

	if err := database.UpdateBuilderTTL(config.Name, config.BuilderTTL); err != nil {
		return nil, fmt.Errorf("error while updating builderTTL for %s : %w", config.Name, err)
	}

	return &pb.SaveConfigResponse{Config: config}, nil
}

// GetConfigById is a gRPC service: function returns one config from the DB based on the requested index/name
func (*server) GetConfigFromDB(ctx context.Context, req *pb.GetConfigFromDBRequest) (*pb.GetConfigFromDBResponse, error) {
	log.Info().Msgf("CLIENT REQUEST: GetConfigFromDB for %s", req.Id)
	config, err := database.GetConfig(req.Id, req.Type)
	if err != nil {
		return nil, fmt.Errorf("error while getting a config %s from database : %w", req.Id, err)
	}
	return &pb.GetConfigFromDBResponse{Config: config}, nil
}

// GetConfigScheduler is a gRPC service: function returns oldest config from the queueScheduler
func (*server) GetConfigScheduler(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	configInfo := queueScheduler.Dequeue()
	if configInfo != nil {
		config, err := database.GetConfig(configInfo.GetName(), pb.IdType_NAME)
		if err != nil {
			return nil, err
		}
		return &pb.GetConfigResponse{Config: config}, nil
	}
	return &pb.GetConfigResponse{Config: nil}, nil
}

// GetConfigBuilder is a gRPC service: function returns oldest config from the queueBuilder
func (*server) GetConfigBuilder(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	configInfo := queueBuilder.Dequeue()
	if configInfo != nil {
		config, err := database.GetConfig(configInfo.GetName(), pb.IdType_NAME)
		if err != nil {
			return nil, err
		}
		return &pb.GetConfigResponse{Config: config}, nil
	}
	return &pb.GetConfigResponse{Config: nil}, nil
}

// GetAllConfigs is a gRPC service: function returns all configs from the DB
func (*server) GetAllConfigs(ctx context.Context, req *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
	configs, err := database.GetAllConfigs()
	if err != nil {
		return nil, fmt.Errorf("error getting all configs : %w", err)
	}
	return &pb.GetAllConfigsResponse{Configs: configs}, nil
}

// DeleteConfig sets the manifest to nil so that the iteration workflow for this
// config destroys the previous build infrastructure.
func (*server) DeleteConfig(ctx context.Context, req *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	log.Info().Msgf("CLIENT REQUEST: DeleteConfig %s", req.Id)
	err := database.UpdateMsToNull(req.Id)
	if err != nil {
		return nil, err
	}

	return &pb.DeleteConfigResponse{Id: req.GetId()}, nil
}

// DeleteConfigFromDB removes the config from the request from the mongoDB database.
func (*server) DeleteConfigFromDB(ctx context.Context, req *pb.DeleteConfigRequest) (*pb.DeleteConfigResponse, error) {
	log.Info().Msgf("CLIENT REQUEST: DeleteConfigFromDB for %s", req.Id)
	if err := database.DeleteConfig(req.GetId(), req.GetType()); err != nil {
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
	log.Info().Msgf("Connected to database at %s", envs.DatabaseURL)
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

	// listen for system interrupts to gracefully shut down
	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)

		// wait for either the received signal or
		// check if an error occurred in other
		// go-routines.
		var err error
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case sig := <-ch:
			log.Info().Msgf("Received signal %v", sig)
			err = errors.New("context-box interrupt signal")
		}

		log.Info().Msg("Gracefully shutting down gRPC server")
		s.GracefulStop()

		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)

		return err
	})

	// server goroutine
	g.Go(func() error {
		// s.Serve() will create a service goroutine for each connection
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("ContextBox failed to serve: %w", err)
		}
		log.Info().Msg("Finished listening for incoming connections")
		return nil
	})

	//config checker go routine
	g.Go(func() error {
		w := worker.NewWorker(ctx, 10*time.Second, configChecker, worker.ErrorLogger)
		w.Run()
		log.Info().Msg("Exited worker loop")
		return nil
	})

	log.Info().Msgf("Stopping Context-Box: %v", g.Wait())
}
