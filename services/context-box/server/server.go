package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/Berops/platform/envs"
	"github.com/Berops/platform/services/context-box/server/checksum"
	claudieDB "github.com/Berops/platform/services/context-box/server/claudieDB"
	kuber "github.com/Berops/platform/services/kuber/client"
	terraformer "github.com/Berops/platform/services/terraformer/client"
	"github.com/Berops/platform/utils"
	"github.com/Berops/platform/worker"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedContextBoxServiceServer
}

type queue struct {
	configs []*configItem
}

type ClaudieDB interface {
	Connect() error
	Disconnect() error
	Init() error
	GetConfig(id string, idType pb.IdType) (*pb.Config, error)
	DeleteConfig(id string, idType pb.IdType) error
	GetAllConfigs() ([]*pb.Config, error)
	SaveConfig(config *pb.Config) error
}

const (
	defaultContextBoxPort = 50055
	defaultBuilderTTL     = 360
	defaultSchedulerTTL   = 5
	defaultAttempts       = 10
	defaultPingTimeout    = 6 * time.Second
)

var (
	database       ClaudieDB
	queueScheduler queue
	queueBuilder   queue
	mutexDBsave    sync.Mutex
)

type configItem struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Name         string             `bson:"name"`
	Manifest     string             `bson:"manifest"`
	DesiredState []byte             `bson:"desiredState"`
	CurrentState []byte             `bson:"currentState"`
	MsChecksum   []byte             `bson:"msChecksum"`
	DsChecksum   []byte             `bson:"dsChecksum"`
	CsChecksum   []byte             `bson:"csChecksum"`
	BuilderTTL   int                `bson:"BuilderTTL"`
	SchedulerTTL int                `bson:"SchedulerTTL"`
	ErrorMessage string             `bson:"errorMessage"`
}

func (q *queue) contains(item *configItem) bool {
	if len(q.configs) == 0 {
		return false
	}
	for _, config := range q.configs {
		if config.Name == item.Name {
			return true
		}
	}
	return false
}

func (q *queue) push() (item *configItem, newQueue queue) {
	if len(q.configs) == 0 {
		return nil, *q
	}
	return q.configs[0], queue{
		configs: q.configs[1:],
	}
}

func configCheck() error {
	mutexDBsave.Lock()
	configs, err := getAllFromDB()
	if err != nil {
		return err
	}
	// loop through config
	for _, config := range configs {
		// check if item is already in some queue
		if queueBuilder.contains(config) || queueScheduler.contains(config) {
			// some queue already has this particular config
			continue
		}

		// check for Scheduler

		if string(config.DsChecksum) != string(config.MsChecksum) {
			// if scheduler ttl is 0 or smaller AND config has no errorMessage, add to scheduler Q
			if config.SchedulerTTL <= 0 && len(config.ErrorMessage) == 0 {
				config.SchedulerTTL = defaultSchedulerTTL

				c, err := dataToConfigPb(config)
				if err != nil {
					return err
				}

				if _, err := saveToDB(c); err != nil {
					return err
				}
				queueScheduler.configs = append(queueScheduler.configs, config)
				continue
			} else {
				config.SchedulerTTL = config.SchedulerTTL - 1
			}
		}

		// check for Builder
		if string(config.DsChecksum) != string(config.CsChecksum) {
			// if builder ttl is 0 or smaller AND config has no errorMessage, add to builder Q
			if config.BuilderTTL <= 0 && len(config.ErrorMessage) == 0 {
				config.BuilderTTL = defaultBuilderTTL

				c, err := dataToConfigPb(config)
				if err != nil {
					return err
				}

				if _, err := saveToDB(c); err != nil {
					return err
				}
				queueBuilder.configs = append(queueBuilder.configs, config)
				continue

			} else {
				config.BuilderTTL = config.BuilderTTL - 1
			}
		}

		// save data if both TTL were subtracted
		c, err := dataToConfigPb(config)
		if err != nil {
			return err
		}

		if _, err := saveToDB(c); err != nil {
			return nil
		}
	}
	mutexDBsave.Unlock()
	return nil
}

func (*server) SaveConfigScheduler(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Info().Msg("CLIENT REQUEST: SaveConfigScheduler")
	config := req.GetConfig()
	err := utils.CheckLengthOfFutureDomain(config)
	if err != nil {
		return nil, fmt.Errorf("error while checking the length of future domain: %v", err)
	}
	// Get config with the same ID from the DB
	data, err := database.GetConfig(config.GetName(), pb.IdType_NAME)
	if err != nil {
		return nil, err
	}
	if !checksum.CompareChecksums(config.MsChecksum, data.MsChecksum) {
		return nil, fmt.Errorf("MsChecksum are not equal")
	}

	// Save new config to the DB
	config.DsChecksum = config.MsChecksum
	config.SchedulerTTL = defaultSchedulerTTL
	mutexDBsave.Lock()
	err = database.SaveConfig(config)
	mutexDBsave.Unlock()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}

	return &pb.SaveConfigResponse{Config: config}, nil
}

func (*server) SaveConfigFrontEnd(ctx context.Context, req *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	log.Info().Msg("CLIENT REQUEST: SaveConfigFrontEnd")
	newConfig := req.GetConfig()
	newConfig.MsChecksum = checksum.CalculateChecksum(newConfig.Manifest)

	oldConfig, err := database.GetConfig(newConfig.GetName(), pb.IdType_NAME)
	if err != nil {
		log.Info().Msgf("No existing doc with name: %v", newConfig.Name)
	} else {
		// copy current state from saved config to new config
		if err != nil {
			log.Fatal().Msgf("Error while converting data to pb %v", err)
		}
		if string(oldConfig.MsChecksum) != string(newConfig.MsChecksum) {
			oldConfig.MsChecksum = newConfig.MsChecksum
			oldConfig.Manifest = newConfig.Manifest
			oldConfig.SchedulerTTL = 0
			oldConfig.BuilderTTL = 0
		}
		newConfig = oldConfig
	}

	// save config to DB
	mutexDBsave.Lock()
	err = database.SaveConfig(newConfig)
	mutexDBsave.Unlock()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
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
	if !checksum.CompareChecksums(config.MsChecksum, databaseConfig.MsChecksum) {
		return nil, fmt.Errorf("msChecksums are not equal")
	}

	// Save new config to the DB
	config.CsChecksum = config.DsChecksum
	config.BuilderTTL = defaultBuilderTTL
	mutexDBsave.Lock()
	err = database.SaveConfig(config)
	mutexDBsave.Unlock()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			fmt.Sprintf("Internal error: %v", err),
		)
	}
	return &pb.SaveConfigResponse{Config: config}, nil
}

// GetConfigById is a gRPC service: function returns one config from the DB
func (*server) GetConfigFromDB(ctx context.Context, req *pb.GetConfigFromDBRequest) (*pb.GetConfigFromDBResponse, error) {
	log.Info().Msg("CLIENT REQUEST: GetConfigFromDB")
	config, err := database.GetConfig(req.Id, req.Type)
	if err != nil {
		log.Error().Msgf("Error while getting a config from database : %v", err)
		return nil, err
	}
	return &pb.GetConfigFromDBResponse{Config: config}, nil
}

// GetConfigScheduler is a gRPC service: function returns one config from the queueScheduler
func (*server) GetConfigScheduler(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Info().Msg("GetConfigScheduler request")
	if len(queueScheduler.configs) > 0 {
		var config *configItem
		config, queueScheduler = queueScheduler.push()
		c, err := dataToConfigPb(config)
		if err != nil {
			return nil, err
		}

		return &pb.GetConfigResponse{Config: c}, nil
	}
	return &pb.GetConfigResponse{}, nil
}

// GetConfigBuilder is a gRPC service: function returns one config from the queueScheduler
func (*server) GetConfigBuilder(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	log.Info().Msg("GetConfigBuilder request")
	if len(queueBuilder.configs) > 0 {
		var config *configItem
		config, queueBuilder = queueBuilder.push()
		c, err := dataToConfigPb(config)
		if err != nil {
			return nil, err
		}

		return &pb.GetConfigResponse{Config: c}, nil
	}
	return &pb.GetConfigResponse{}, nil
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
	// find a config from database
	config, err := database.GetConfig(req.Id, req.Type)
	if err != nil {
		return nil, err
	}
	//destroy infrastructure with terraformer
	_, err = destroyConfigTerraformer(config)
	if err != nil {
		return nil, err
	}
	// delete kubeconfig secret
	err = deleteKubeconfig(config)
	if err != nil {
		return nil, err
	}
	//delete config from db
	err = database.DeleteConfig(req.Id, req.Type)
	if err != nil {
		return nil, err
	}
	return &pb.DeleteConfigResponse{Id: req.GetId()}, nil
}

// destroyConfigTerraformer calls terraformer's DestroyInfrastructure function
func destroyConfigTerraformer(config *pb.Config) (*pb.Config, error) {
	// Trim "tcp://" substring from envs.TerraformerURL
	trimmedTerraformerURL := strings.ReplaceAll(envs.TerraformerURL, ":tcp://", "")
	log.Info().Msgf("Dial Terraformer: %s", trimmedTerraformerURL)
	// Create connection to Terraformer
	cc, err := utils.GrpcDialWithInsecure("terraformer", trimmedTerraformerURL)
	if err != nil {
		return nil, err
	}
	defer func() { utils.CloseClientConnection(cc) }()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	res, err := terraformer.DestroyInfrastructure(c, &pb.DestroyInfrastructureRequest{Config: config})
	if err != nil {
		return nil, err
	}

	return res.GetConfig(), nil
}

// gRPC call to delete
func deleteKubeconfig(config *pb.Config) error {
	trimmedKuberURL := strings.ReplaceAll(envs.KuberURL, ":tcp://", "")
	log.Info().Msgf("Dial Terraformer: %s", trimmedKuberURL)
	// Create connection to Terraformer
	cc, err := utils.GrpcDialWithInsecure("kuber", trimmedKuberURL)
	if err != nil {
		return err
	}
	defer func() { utils.CloseClientConnection(cc) }()

	c := pb.NewKuberServiceClient(cc)
	for _, cluster := range config.CurrentState.Clusters {
		_, err := kuber.DeleteKubeconfig(c, &pb.DeleteKubeconfigRequest{Cluster: cluster})
		if err != nil {
			return err
		}
	}
	return nil
}

func configChecker() error {
	if err := configCheck(); err != nil {
		return fmt.Errorf("error while configCheck: %v", err)
	}
	log.Info().Msgf("QueueScheduler content: %v", queueScheduler)
	log.Info().Msgf("QueueBuilder content: %v", queueBuilder)
	return nil
}

func initDatabase() (ClaudieDB, error) {
	claudieDatabase := &claudieDB.ClaudieMongo{Url: envs.DatabaseURL}
	err := claudieDatabase.Connect()
	if err != nil {
		log.Error().Msgf("Unable to connect to database at %s : %v", envs.DatabaseURL, err)
		return nil, err
	}
	log.Info().Msgf("Connected to database at %s", envs.DatabaseURL)
	err = claudieDatabase.Init()
	if err != nil {
		log.Error().Msgf("Unable to initialise to database at %s : %v", envs.DatabaseURL, err)
		return nil, err
	}
	return claudieDatabase, nil
}

func main() {
	// initialize logger & db
	utils.InitLog("context-box")
	var err error
	database, err = initDatabase()
	if err != nil {
		log.Fatal().Msgf("Failed to connect to the database, aborting... : %v", err)
	}
	defer database.Disconnect()

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
