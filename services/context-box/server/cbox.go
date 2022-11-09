package main

import (
	"fmt"

	"github.com/Berops/claudie/internal/envs"
	"github.com/Berops/claudie/proto/pb"
	"github.com/Berops/claudie/services/context-box/server/checksum"
	"github.com/Berops/claudie/services/context-box/server/claudieDB"
	"github.com/Berops/claudie/services/context-box/server/queue"
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
	UpdateMsToNull(id string) error
	UpdateDs(config *pb.Config) error
	UpdateCs(config *pb.Config) error
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

// getConfigInfos returns a slice of ConfigInfos based on the configs currently in database
func getConfigInfos() ([]*ConfigInfo, error) {
	configs, err := database.GetAllConfigs()
	if err != nil {
		return nil, err
	}
	var result []*ConfigInfo
	for _, config := range configs {
		configInfo := &ConfigInfo{
			Name:         config.Name,
			MsChecksum:   config.MsChecksum,
			CsChecksum:   config.CsChecksum,
			DsChecksum:   config.DsChecksum,
			BuilderTTL:   config.BuilderTTL,
			SchedulerTTL: config.SchedulerTTL,
			ErrorMessage: config.ErrorMessage,
		}
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
		if !checksum.CompareChecksums(config.DsChecksum, config.MsChecksum) {
			// if scheduler ttl is 0 or smaller AND config has no errorMessage, add to scheduler Q
			if config.SchedulerTTL <= 0 && len(config.ErrorMessage) == 0 {
				if err := database.UpdateSchedulerTTL(config.Name, defaultSchedulerTTL); err != nil {
					return err
				}
				queueScheduler.Enqueue(config)
				config.SchedulerTTL = defaultSchedulerTTL
				continue
			} else {
				config.SchedulerTTL = config.SchedulerTTL - 1
			}
		}

		// check for Builder
		if !checksum.CompareChecksums(config.DsChecksum, config.CsChecksum) {
			// if builder ttl is 0 or smaller AND config has no errorMessage, add to builder Q
			if config.BuilderTTL <= 0 && len(config.ErrorMessage) == 0 {
				if err := database.UpdateBuilderTTL(config.Name, defaultBuilderTTL); err != nil {
					return err
				}
				queueBuilder.Enqueue(config)
				config.BuilderTTL = defaultBuilderTTL
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

// configChecker is a driver for configCheck function
func configChecker() error {
	if err := configCheck(); err != nil {
		return fmt.Errorf("error while configCheck: %v", err)
	}
	qs := queueScheduler.GetContent()
	qb := queueBuilder.GetContent()
	if len(qs) > 0 {
		log.Info().Msgf("QueueScheduler content: %v", qs)
	}
	if len(qb) > 0 {
		log.Info().Msgf("QueueBuilder content: %v", qb)
	}
	return nil
}

// initDatabase will establish connection to the DB and initialise it to our needs, i.e. creates collections, etc..
func initDatabase() (ClaudieDB, error) {
	claudieDatabase := &claudieDB.ClaudieMongo{URL: envs.DatabaseURL}
	err := claudieDatabase.Connect()
	if err != nil {
		return nil, fmt.Errorf("Unable to connect to database at %s : %v", envs.DatabaseURL, err)
	}
	err = claudieDatabase.Init()
	if err != nil {
		return nil, fmt.Errorf("Unable to initialise to database at %s : %v", envs.DatabaseURL, err)
	}
	return claudieDatabase, nil
}
