package main

import (
	"fmt"
	"strings"

	"github.com/Berops/platform/envs"
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/context-box/server/claudieDB"
	"github.com/Berops/platform/services/context-box/server/queue"
	kuber "github.com/Berops/platform/services/kuber/client"
	terraformer "github.com/Berops/platform/services/terraformer/client"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
)

type ClaudieDB interface {
	Connect() error
	Disconnect() error
	Init() error
	GetConfig(id string, idType pb.IdType) (*pb.Config, error)
	DeleteConfig(id string, idType pb.IdType) error
	GetAllConfigs() ([]*pb.Config, error)
	SaveConfig(config *pb.Config) error
	UpdateSchedulerTTL(name string, newTTL int32) error
	UpdateBuilderTTL(name string, newTTL int32) error
}

type ConfigInfo struct {
	Name         string
	MsChecksum   []byte
	CsChecksum   []byte
	DsChecksum   []byte
	BuilderTTL   int32
	SchedulerTTL int32
	ErrorMessage string
}

const (
	defaultBuilderTTL   = 360
	defaultSchedulerTTL = 5
)

var (
	queueScheduler queue.Queue
	queueBuilder   queue.Queue
)

func (ci *ConfigInfo) GetName() string {
	return ci.Name
}

func getConfigInfos() ([]*ConfigInfo, error) {
	configs, err := database.GetAllConfigs()
	if err != nil {
		return nil, err
	}
	var result []*ConfigInfo
	for _, config := range configs {
		configInfo := &ConfigInfo{Name: config.Name, MsChecksum: config.MsChecksum, CsChecksum: config.CsChecksum, DsChecksum: config.DsChecksum,
			BuilderTTL: config.BuilderTTL, SchedulerTTL: config.SchedulerTTL, ErrorMessage: config.ErrorMessage}
		result = append(result, configInfo)
	}
	return result, nil
}

func configCheck() error {
	configs, err := getConfigInfos()
	if err != nil {
		return err
	}
	// loop through config
	for _, config := range configs {
		// check if item is already in some queue
		if queueBuilder.Contains(config) || queueScheduler.Contains(config) {
			// some queue already has this particular config
			continue
		}

		// check for Scheduler
		if string(config.DsChecksum) != string(config.MsChecksum) {
			// if scheduler ttl is 0 or smaller AND config has no errorMessage, add to scheduler Q
			if config.SchedulerTTL <= 0 && len(config.ErrorMessage) == 0 {
				if err := database.UpdateSchedulerTTL(config.Name, defaultSchedulerTTL); err != nil {
					return err
				}
				queueScheduler.Enqueue(config)
				continue
			} else {
				config.SchedulerTTL = config.SchedulerTTL - 1
			}
		}

		// check for Builder
		if string(config.DsChecksum) != string(config.CsChecksum) {
			// if builder ttl is 0 or smaller AND config has no errorMessage, add to builder Q
			if config.BuilderTTL <= 0 && len(config.ErrorMessage) == 0 {
				if err := database.UpdateBuilderTTL(config.Name, defaultBuilderTTL); err != nil {
					return err
				}
				queueBuilder.Enqueue(config)
				continue

			} else {
				config.BuilderTTL = config.BuilderTTL - 1
			}
		}

		// save data if both TTL were subtracted
		if err := database.UpdateSchedulerTTL(config.Name, config.SchedulerTTL); err != nil {
			return nil
		}
		if err := database.UpdateBuilderTTL(config.Name, config.BuilderTTL); err != nil {
			return nil
		}
	}
	return nil
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
