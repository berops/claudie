package usecases

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

// ConfigProcessor will fetch new config from the context-box microservice.
// Each received config will be processed in a separate go-routine.
// If a sync.WaitGroup is supplied it will call the Add(1) and then the Done() method on it after
// the go-routine finishes the work, if nil it will be ignored.
func (u *Usecases) ConfigProcessor(contextBoxGrpcClient pb.ContextBoxServiceClient, waitGroup *sync.WaitGroup) error {
	// Pull an item from the context-box scheduler queue
	response, err := u.ContextBox.GetConfigScheduler(contextBoxGrpcClient)
	if err != nil {
		return fmt.Errorf("error while getting Scheduler config: %w", err)
	}

	config := response.GetConfig()
	if config == nil {
		return nil
	}

	if waitGroup != nil {
		// We received a non-nil config thus we add a new worker to the wait group.
		waitGroup.Add(1)
	}

	go func() {
		if waitGroup != nil {
			defer waitGroup.Done()
		}

		log.Info().Msgf("Processing config %s ", config.Name)

		// Process (build desired state) the config
		if configProcessingErr := u.processConfig(config, contextBoxGrpcClient); configProcessingErr != nil {
			log.Error().Msgf("Error while processing config %s : %v", config.Name, configProcessingErr)

			// Save processing error message to config
			if err := u.saveErrorMessageToConfig(config, contextBoxGrpcClient, configProcessingErr); err != nil {
				log.Error().Msgf("Failed to save error to the config %s : %v", config.Name, err)
			}
		}
		log.Info().Msgf("Config %s have been successfully processed", config.Name)
	}()

	return nil
}

// processConfig contains the core logic of processing a config
// returns error if not successful, nil otherwise
func (u *Usecases) processConfig(config *pb.Config, contextBoxGrpcClient pb.ContextBoxServiceClient) error {
	// Create desired state
	config, err := u.CreateDesiredState(config)
	if err != nil {
		return fmt.Errorf("error while creating a desired state: %w", err)
	}

	// After constructing the desired state for the config
	// save it to the context-box DB
	err = u.ContextBox.SaveConfigScheduler(config, contextBoxGrpcClient)
	if err != nil {
		return fmt.Errorf("error while saving the config: %w", err)
	}

	return nil
}

// saveErrorMessageToConfig saves error message to the config
// Returns error if not successful, nil otherwise
func (u *Usecases) saveErrorMessageToConfig(config *pb.Config, contextBoxGrpcClient pb.ContextBoxServiceClient, err error) error {
	// TODO: Investigate this line - @MiroslavRepka
	config.CurrentState = config.DesiredState // Update CurrentState, so we can use it for deletion later

	if config.State == nil {
		config.State = make(map[string]*pb.Workflow)
	}

	config.State["scheduler"] = &pb.Workflow{
		Stage:       pb.Workflow_SCHEDULER,
		Status:      pb.Workflow_ERROR,
		Description: err.Error(),
	}

	return u.ContextBox.SaveConfigScheduler(config, contextBoxGrpcClient)
}
