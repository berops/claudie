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

// ClaudieDB interface describes functionality that cbox needs in order to function properly
// By using an interface, we abstract the underlying DB plus, it can be changed by simply providing different interface implementation
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

// ConfigInfo struct describes data which cbox needs to hold in order to function properly
// it is used in configChecker, to decide which configs should be enqueued and which not
// it must implement ConfigItem interface in order to be used with queue package
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

// GetName is function required by queue package to evaluate equivalence
func (ci *ConfigInfo) GetName() string {
	return ci.Name
}

//getConfigInfos returns a slice of ConfigInfos based on the configs currently in database
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

// configCheck function checks all configs, decides if they should be enqueued and updates their TTLs
func configCheck() error {
	configs, err := getConfigInfos()
	if err != nil {
		return err
	}
	// loop through configInfos from db
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
func destroyConfigTerraformer(config *pb.Config) error {
	// Trim "tcp://" substring from envs.TerraformerURL
	trimmedTerraformerURL := strings.ReplaceAll(envs.TerraformerURL, ":tcp://", "")
	log.Info().Msgf("Dial Terraformer: %s", trimmedTerraformerURL)
	// Create connection to Terraformer
	cc, err := utils.GrpcDialWithInsecure("terraformer", trimmedTerraformerURL)
	if err != nil {
		return err
	}
	defer func() { utils.CloseClientConnection(cc) }()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	_, err = terraformer.DestroyInfrastructure(c, &pb.DestroyInfrastructureRequest{Config: config})
	if err != nil {
		return err
	}
	return nil
}

// deleteKubeconfig calls kuber's DeleteKubeconfig function
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

// configChecker is a driver for configCheck function
func configChecker() error {
	if err := configCheck(); err != nil {
		return fmt.Errorf("error while configCheck: %v", err)
	}
	log.Info().Msgf("QueueScheduler content: %v", queueScheduler)
	log.Info().Msgf("QueueBuilder content: %v", queueBuilder)
	return nil
}

// initDatabase will establish connection to the DB and initialise it to our needs, i.e. creates collections, etc..
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
