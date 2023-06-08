package usecases

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
	outboundAdapters "github.com/berops/claudie/services/context-box/server/adapters/outbound"
	"github.com/berops/claudie/services/context-box/server/domain/ports"
	"github.com/berops/claudie/services/context-box/server/utils"
)

const (
	// default TTL for an element to be in the builder queue
	defaultBuilderTTL = 360

	// default TTL for an element to be in the scheduler queue
	defaultSchedulerTTL = 5
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

// hasAnyError returns true if any cluster errored while building or destroying.
func (c *ConfigInfo) hasAnyError() bool {
	for _, v := range c.State {
		if v.Status == pb.Workflow_ERROR.String() {
			return true
		}
	}

	return false
}

// hasDestroyError returns true if error occurred in any of the clusters
// while getting destroyed.
func (c *ConfigInfo) hasDestroyError() bool {
	for _, v := range c.State {
		if v.Status == pb.Workflow_ERROR.String() && strings.Contains(v.Stage, "DESTROY") {
			return true
		}
	}
	return false
}

// scheduleDeletion returns true if config should be pushed onto any queue due to deletion.
// This ignores the build errors, as we want to remove infrastructure if secret was deleted,
// However, respects error from destroy workflow, as we do not want to retry indefinitely.
func (c *ConfigInfo) scheduleDeletion() bool {
	// Ignore as deletion already errored out
	if c.hasDestroyError() {
		return false
	}

	// Scheduler queue
	if c.MsChecksum == nil && c.DsChecksum != nil {
		return true
	}
	// Builder queue
	if c.MsChecksum == nil && c.DsChecksum == nil && c.CsChecksum != nil {
		return true
	}

	// Not scheduled for deletion
	return false
}

// EnqueueConfigs is a driver for enqueueConfigs function
func (u *Usecases) EnqueueConfigs() error {
	if err := u.enqueueConfigs(); err != nil {
		return fmt.Errorf("error while enqueuing configs: %w", err)
	}

	if !u.schedulerQueue.CompareElementNameList(u.schedulerLogQueue) {
		log.Info().Msgf("Scheduler queue content changed to: %v", u.schedulerQueue.GetElementNames())
	}
	u.schedulerLogQueue = u.schedulerQueue.GetElementNames()

	if !u.builderQueue.CompareElementNameList(u.builderLogQueue) {
		log.Info().Msgf("Builder queue content changed to: %v", u.builderQueue.GetElementNames())
	}
	u.builderLogQueue = u.builderQueue.GetElementNames()

	return nil
}

// enqueueConfigs checks all configs, decides if they should be enqueued or not and updates their TTLs
func (u *Usecases) enqueueConfigs() error {
	configInfos, err := getConfigInfosFromDB(u.DB)
	if err != nil {
		return err
	}

	for _, configInfo := range configInfos {
		// If item is already in some queue (scheduler / builder) then skip and move to the next item
		if u.builderQueue.Contains(configInfo) || u.schedulerQueue.Contains(configInfo) {
			continue
		}

		// Initially when the config is received from the frontend microservice, the desired state of the config is not built,
		// due to which the DsChecksum will be nil.
		if !utils.Equal(configInfo.DsChecksum, configInfo.MsChecksum) {
			// If scheduler TTL is <= 0 AND config has no errorMessage, add item to the scheduler queue
			if configInfo.SchedulerTTL <= 0 {
				if !configInfo.hasAnyError() || configInfo.scheduleDeletion() {
					if err := u.DB.UpdateSchedulerTTL(configInfo.Name, defaultSchedulerTTL); err != nil {
						return err
					}

					// The item is put in the scheduler queue. The scheduler microservice will eventually pull the corresponding
					// config and build its desired state
					u.schedulerQueue.Enqueue(configInfo)
					configInfo.SchedulerTTL = defaultSchedulerTTL

					continue
				} else if !configInfo.hasAnyError() {
					// If the item is already present in the scheduler queue but the config is still not pulled by the scheduler
					// microservice, then reduce its scheduler TTL by 1.
					configInfo.SchedulerTTL = configInfo.SchedulerTTL - 1
				}
			}
		}

		// After the config has its desired state built, the infrastructure needs to be provisioned. The DsChecksum and CsChecksum
		// doesn't match since the infrastructure is not built yet.
		if !utils.Equal(configInfo.DsChecksum, configInfo.CsChecksum) {
			// If builder TTL <= 0 AND config has no errorMessage, add item to the builder queue
			if configInfo.BuilderTTL <= 0 {
				// If no BUILD error OR if triggered for deletion in builder microservice.
				if !configInfo.hasAnyError() || configInfo.scheduleDeletion() {
					if err := u.DB.UpdateBuilderTTL(configInfo.Name, defaultBuilderTTL); err != nil {
						return err
					}

					// The item is put in the builder queue. The builder microservice will eventually pull the corresponding
					// config and provision the corresponding infrastructure.
					u.builderQueue.Enqueue(configInfo)
					configInfo.BuilderTTL = defaultBuilderTTL

					continue
				}
			} else if !configInfo.hasAnyError() {
				// If the item is already present in the builder queue but the config is still not pulled by the builder
				// microservice, then reduce its scheduler TTL by 1.
				configInfo.BuilderTTL = configInfo.BuilderTTL - 1
			}
		}

		// save data if both TTL were subtracted
		if err := u.DB.UpdateSchedulerTTL(configInfo.Name, configInfo.SchedulerTTL); err != nil {
			return err
		}
		if err := u.DB.UpdateBuilderTTL(configInfo.Name, configInfo.BuilderTTL); err != nil {
			return err
		}
	}

	return nil
}

// Fetches all configAsBSON from MongoDB and converts each configAsBSON to ConfigInfo
// Then returns the list
func getConfigInfosFromDB(mongoDB ports.DBPort) ([]*ConfigInfo, error) {
	configs, err := mongoDB.GetAllConfigs()
	if err != nil {
		return nil, err
	}

	var configInfos []*ConfigInfo

	for _, configAsBSON := range configs {
		configInfo := &ConfigInfo{
			Name: configAsBSON.Name,

			MsChecksum: configAsBSON.MsChecksum,
			CsChecksum: configAsBSON.CsChecksum,
			DsChecksum: configAsBSON.DsChecksum,

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
