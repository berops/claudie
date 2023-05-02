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

const (
	// default TTL for an element to be in the builder queue
	defaultBuilderTTL = 360

	// default TTL for an element to be in the scheduler queue
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

		// Initially when the config is received from the frontend microservice, the desired state of the config is not built,
		// due to which the DsChecksum will be nil.
		if !utils.Equal(configInfo.DsChecksum, configInfo.MsChecksum) {
			// If scheduler TTL is <= 0 AND config has no errorMessage, add item to the scheduler queue
			if configInfo.SchedulerTTL <= 0 && !configInfo.HasError() {
				if err := u.DB.UpdateSchedulerTTL(configInfo.Name, defaultSchedulerTTL); err != nil {
					return err
				}

				// The item is put in the scheduler queue. The scheduler microservice will eventually pull the corresponding
				// config and build its desired state
				u.schedulerQueue.Enqueue(configInfo)
				configInfo.SchedulerTTL = defaultSchedulerTTL

				continue
			}

			// If the item is already present in the scheduler queue but the config is still not pulled by the scheduler
			// microservice, then reduce its scheduler TTL by 1.
			configInfo.SchedulerTTL = configInfo.SchedulerTTL - 1
		}

		// After the config has its desired state built, the infrastructure needs to be provisioned. The DsChecksum and CsChecksum
		// doesn't match since the infrastructure is not built yet.
		if !utils.Equal(configInfo.DsChecksum, configInfo.CsChecksum) {
			// If builder TTL <= 0 AND config has no errorMessage, add item to the builder queue
			if configInfo.BuilderTTL <= 0 && !configInfo.HasError() {
				if err := u.DB.UpdateBuilderTTL(configInfo.Name, defaultBuilderTTL); err != nil {
					return err
				}

				// The item is put in the builder queue. The builder microservice will eventually pull the corresponding
				// config and provision the corresponding infrastructure.
				u.builderQueue.Enqueue(configInfo)
				configInfo.BuilderTTL = defaultBuilderTTL

				continue
			}

			// If the item is already present in the builder queue but the config is still not pulled by the builder
			// microservice, then reduce its scheduler TTL by 1.
			configInfo.BuilderTTL = configInfo.BuilderTTL - 1
		}

		// save data if both TTL were subtracted
		if err := u.DB.UpdateSchedulerTTL(configInfo.Name, configInfo.SchedulerTTL); err != nil {
			break
		}
		if err := u.DB.UpdateBuilderTTL(configInfo.Name, configInfo.BuilderTTL); err != nil {
			break
		}
	}

	if !u.builderQueue.CompareElementNameList(u.builderLogQueue) {
		log.Info().Msgf("Builder queue content changed to: %v", u.builderQueue.GetElementNames())
	}
	u.builderLogQueue = u.builderQueue.GetElementNames()

	if !u.schedulerQueue.CompareElementNameList(u.schedulerLogQueue) {
		log.Info().Msgf("Scheduler queue content changed to: %v", u.schedulerQueue.GetElementNames())
	}
	u.schedulerLogQueue = u.schedulerQueue.GetElementNames()

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
