package main

import (
	"fmt"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/context-box/server/checksum"
	"github.com/berops/claudie/services/context-box/server/claudieDB"
	"github.com/berops/claudie/services/context-box/server/queue"
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
	UpdateWorkflowState(configName, clusterName string, workflow *pb.Workflow) error
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
	State        map[string]claudieDB.Workflow
}

const (
	defaultBuilderTTL   = 360
	defaultSchedulerTTL = 5
)

var (
	//TODO move them to server struct
	queueScheduler queue.Queue
	queueBuilder   queue.Queue
	//vars used for logging
	lastQB = []string{}
	lastQS = []string{}
)

// HasError returns true if any cluster errored while building.
func (ci *ConfigInfo) HasError() bool {
	for _, v := range ci.State {
		if v.Status == pb.Workflow_ERROR.String() {
			return true
		}
	}
	return false
}

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
			State: func() map[string]claudieDB.Workflow {
				state := make(map[string]claudieDB.Workflow)
				for key, val := range config.State {
					state[key] = claudieDB.Workflow{
						Status:      val.Status.String(),
						Stage:       val.Stage.String(),
						Description: val.Description,
					}
				}
				return state
			}(),
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
		if !checksum.Equals(config.DsChecksum, config.MsChecksum) {
			// if scheduler ttl is 0 or smaller AND config has no errorMessage, add to scheduler Q
			if config.SchedulerTTL <= 0 && !config.HasError() {
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
		if !checksum.Equals(config.DsChecksum, config.CsChecksum) {
			// if builder ttl is 0 or smaller AND config has no errorMessage, add to builder Q
			if config.BuilderTTL <= 0 && !config.HasError() {
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
		return fmt.Errorf("error while configCheck: %w", err)
	}
	if !queueScheduler.CompareContent(lastQS) {
		log.Info().Msgf("QueueScheduler content changed to: %v", queueScheduler.GetContent())
	}
	if !queueBuilder.CompareContent(lastQB) {
		log.Info().Msgf("QueueBuilder content changed to: %v", queueBuilder.GetContent())
	}
	lastQS = queueScheduler.GetContent()
	lastQB = queueBuilder.GetContent()
	return nil
}

// initDatabase will establish connection to the DB and initialise it to our needs, i.e. creates collections, etc..
func initDatabase() (ClaudieDB, error) {
	claudieDatabase := &claudieDB.ClaudieMongo{URL: envs.DatabaseURL}
	err := claudieDatabase.Connect()
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database at %s : %w", envs.DatabaseURL, err)
	}
	err = claudieDatabase.Init()
	if err != nil {
		return nil, fmt.Errorf("unable to initialise to database at %s : %w", envs.DatabaseURL, err)
	}
	return claudieDatabase, nil
}

func updateNodepool(state *pb.Project, clusterName, nodepoolName string, nodes []*pb.Node, count *int32) error {
	for _, cluster := range state.Clusters {
		if cluster.ClusterInfo.Name == clusterName {
			for _, nodepool := range cluster.ClusterInfo.NodePools {
				if nodepool.Name == nodepoolName {
					// Update nodes
					nodepool.Nodes = nodes
					if count != nil {
						nodepool.Count = *count
					}
					return nil
				}
			}
			return fmt.Errorf("nodepool %s was not found in cluster %s", nodepoolName, clusterName)
		}
	}
	return fmt.Errorf("cluster %s was not found in project %s", clusterName, state.Name)
}

// checkStateForError checks if state contains error and if it should be saved. If
// not, returns original state, otherwise error status is deleted and new description appended.
func checkStateForError(saveErrors bool, state *pb.Workflow) *pb.Workflow {
	if !saveErrors {
		if state.Status == pb.Workflow_ERROR {
			state.Status = pb.Workflow_DONE
			state.Description = fmt.Sprintf("Error encountered but ignored due to triggered deletion : %s", state.Description)
		}
	}
	return state
}
