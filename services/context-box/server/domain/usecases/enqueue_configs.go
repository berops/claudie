package usecases

import (
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
	outboundAdapters "github.com/berops/claudie/services/context-box/server/adapters/outbound"
	"github.com/berops/claudie/services/context-box/server/domain/ports"
	"github.com/berops/claudie/services/context-box/server/utils"
)

// ConfigInfo is a data structure which holds data that context-box needs in order to function properly.
// This data is required by EnqueueConfigs to decide which configs should be enqueued and which should not
type ConfigInfo struct {
	Name         string
	MsChecksum   []byte
	CsChecksum   []byte
	DsChecksum   []byte
	BuilderTTL   int32
	SchedulerTTL int32
	State        map[string]outboundAdapters.Workflow
}

// GetName function is required by the queue to evaluate equivalence
func (c *ConfigInfo) GetName() string {
	return c.Name
}

// HasError returns true if any cluster errored while building.
func (c *ConfigInfo) HasError() bool {
	for _, v := range c.State {
		if v.Status == pb.Workflow_ERROR.String() {
			return true
		}
	}

	return false
}

var (
	// Used for logging purposes
	// Logs are generated whenever elements are added/removed to/from the queue
	elementNameListForBuilderQueueWithPreviousState   []string
	elementNameListForSchedulerQueueWithPreviousState []string
)

const (
	defaultBuilderTTL   = 360
	defaultSchedulerTTL = 5
)

// EnqueueConfigs checks all configs, decides if they should be enqueued or not and updates their TTLs
func (u *Usecases) EnqueueConfigs() error {
	configInfos, err := getConfigInfosFromDB(u.DB)
	if err != nil {
		return err
	}

	for _, configInfo := range configInfos {
		// if item is already in some queue (scheduler / builder) then skip and move to the next item
		if u.builderQueue.Contains(configInfo) || u.schedulerQueue.Contains(configInfo) {
			continue
		}

		// TODO: understand trailing code

		// check for Scheduler
		if !utils.CompareChecksum(configInfo.DsChecksum, configInfo.MsChecksum) {
			// if scheduler ttl is 0 or smaller AND config has no errorMessage, add to scheduler Q
			if configInfo.SchedulerTTL <= 0 && !configInfo.HasError() {
				if err := u.DB.UpdateSchedulerTTL(configInfo.Name, defaultSchedulerTTL); err != nil {
					return err
				}
				u.schedulerQueue.Enqueue(configInfo)
				configInfo.SchedulerTTL = defaultSchedulerTTL
				continue
			} else {
				configInfo.SchedulerTTL = configInfo.SchedulerTTL - 1
			}
		}

		// check for Builder
		if !utils.CompareChecksum(configInfo.DsChecksum, configInfo.CsChecksum) {
			// if builder ttl is 0 or smaller AND config has no errorMessage, add to builder Q
			if configInfo.BuilderTTL <= 0 && !configInfo.HasError() {
				if err := u.DB.UpdateBuilderTTL(configInfo.Name, defaultBuilderTTL); err != nil {
					return err
				}
				u.builderQueue.Enqueue(configInfo)
				configInfo.BuilderTTL = defaultBuilderTTL
				continue
			} else {
				configInfo.BuilderTTL = configInfo.BuilderTTL - 1
			}
		}

		// save data if both TTL were subtracted
		if err := u.DB.UpdateSchedulerTTL(configInfo.Name, configInfo.SchedulerTTL); err != nil {
			break
		}
		if err := u.DB.UpdateBuilderTTL(configInfo.Name, configInfo.BuilderTTL); err != nil {
			break
		}
	}

	if !u.builderQueue.CompareElementNameList(elementNameListForBuilderQueueWithPreviousState) {
		log.Info().Msgf("Builder queue with current state has element names: %v", u.builderQueue.GetElementNames())
	}
	elementNameListForBuilderQueueWithPreviousState = u.builderQueue.GetElementNames()

	if !u.schedulerQueue.CompareElementNameList(elementNameListForSchedulerQueueWithPreviousState) {
		log.Info().Msgf("Scheduler queue with current state has element names: %v", u.schedulerQueue.GetElementNames())
	}
	elementNameListForSchedulerQueueWithPreviousState = u.schedulerQueue.GetElementNames()

	return nil
}

// Fetches all configAsBSON from MongoDB and converts each configAsBSON to ConfigInfo
// Then returns the list
func getConfigInfosFromDB(mongoDB ports.DBPort) ([]*ConfigInfo, error) {
	configAsBSONList, err := mongoDB.GetAllConfigs()
	if err != nil {
		return nil, err
	}

	var configInfos []*ConfigInfo

	for _, configAsBSON := range configAsBSONList {
		configInfo := &ConfigInfo{
			Name:         configAsBSON.Name,
			MsChecksum:   configAsBSON.MsChecksum,
			CsChecksum:   configAsBSON.CsChecksum,
			DsChecksum:   configAsBSON.DsChecksum,
			BuilderTTL:   configAsBSON.BuilderTTL,
			SchedulerTTL: configAsBSON.SchedulerTTL,

			State: func() map[string]outboundAdapters.Workflow {
				state := make(map[string]outboundAdapters.Workflow)

				for key, val := range configAsBSON.State {
					state[key] = outboundAdapters.Workflow{
						Status:      val.Status.String(),
						Stage:       val.Stage.String(),
						Description: val.Description,
					}
				}

				return state
			}(),
		}

		configInfos = append(configInfos, configInfo)
	}

	return configInfos, nil
}
