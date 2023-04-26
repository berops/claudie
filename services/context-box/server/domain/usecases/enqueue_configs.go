package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/services/context-box/server/domain/ports"
	"github.com/berops/claudie/services/context-box/server/utils"
)

// SubConfig is a data structure which holds some fields of the Config data structure
// This data is required by EnqueueConfigs to decide which configs should be enqueued and which not
type SubConfig struct {
	Name string

	ManifestChecksum       []byte
	DesiredProjectChecksum []byte
	CurrentProjectChecksum []byte
}

func (s *SubConfig) GetName() string {
	return s.Name
}

var (
	// queue containing configs which needs to be processed by the builder microservice
	builderQueue utils.Queue
	// queue containing configs which needs to be processed by the scheduler microservice
	schedulerQueue utils.Queue

	// Used for logging purposes
	// Logs are generated whenever elements are added/removed to/from the queue
	elementNameListForBuilderQueueWithPreviousState   []string
	elementNameListForSchedulerQueueWithPreviousState []string
)

func (u *Usecases) EnqueueConfigs() error {

	subConfigs, err := getSubConfigsFromDB(u.MongoDB)
	if err != nil {
		return fmt.Errorf("Error while enqueueing configs: %w", err)
	}

	for _, subConfig := range subConfigs {

		if builderQueue.Contains(subConfig) || schedulerQueue.Contains(subConfig) {
			continue
		}

		// TODO: review

		if !utils.CompareChecksum(subConfig.DesiredProjectChecksum, subConfig.ManifestChecksum) {
			schedulerQueue.Enqueue(subConfig)
			continue
		}

		if !utils.CompareChecksum(subConfig.CurrentProjectChecksum, subConfig.ManifestChecksum) {
			builderQueue.Enqueue(subConfig)
			continue
		}
	}

	if !builderQueue.CompareElementNameList(elementNameListForBuilderQueueWithPreviousState) {
		log.Info().Msgf("Builder queue with current state has element names: %v", builderQueue.GetElementNames())
	}
	elementNameListForBuilderQueueWithPreviousState = builderQueue.GetElementNames()

	if !schedulerQueue.CompareElementNameList(elementNameListForBuilderQueueWithPreviousState) {
		log.Info().Msgf("Scheduler queue with current state has element names: %v", schedulerQueue.GetElementNames())
	}
	elementNameListForSchedulerQueueWithPreviousState = schedulerQueue.GetElementNames()

	return nil

}

// Fetches all configAsBSON from MongoDB and converts each configAsBSON to SubConfig
// Then returns the list
func getSubConfigsFromDB(mongoDB ports.MongoDBPort) ([]*SubConfig, error) {
	configAsBSONList, err := mongoDB.GetAllConfigs()
	if err != nil {
		return nil, err
	}

	var subConfigs []*SubConfig

	for _, configAsBSON := range configAsBSONList {

		subConfig := &SubConfig{
			Name:             configAsBSON.Name,
			ManifestChecksum: configAsBSON.MsChecksum,
		}

		subConfigs = append(subConfigs, subConfig)
	}

	return subConfigs, nil
}
