package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/Berops/platform/envs"
	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/utils"
	"github.com/Berops/platform/worker"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

const (
	defaultSchedulerPort = 50056
)

//processConfig is function used to carry out task specific to Scheduler
//returns error if not successful, nil otherwise
func processConfig(config *pb.Config, c pb.ContextBoxServiceClient) error {
	log.Printf("Processing new config")
	//create desired state
	config, err := createDesiredState(config)
	if err != nil {
		return fmt.Errorf("error while creating a desired state: %v", err)
	}

	//save config with new desired state
	err = cbox.SaveConfigScheduler(c, &pb.SaveConfigRequest{Config: config})
	if err != nil {
		return fmt.Errorf("error while saving the config: %v", err)
	}

	return nil
}

//configProcessor is worker function which invokes the processConfig() function
//returns func()error which will carry out the processConfig() execution
func configProcessor(c pb.ContextBoxServiceClient) func() error {
	return func() error {
		//pull an item from a queue in cbox
		res, err := cbox.GetConfigScheduler(c)
		if err != nil {
			return fmt.Errorf("error while getting Scheduler config: %v", err)
		}

		//process config
		config := res.GetConfig()
		if config != nil {
			go func(config *pb.Config) {
				log.Info().Msgf("Processing %s ", config.Name)
				err := processConfig(config, c)
				if err != nil {
					log.Info().Msgf("processConfig() failed: %s", err)
					//save error message to config
					errSave := saveErrorMessage(config, c, err)
					if errSave != nil {
						log.Error().Msgf("scheduler:failed to save error to the config: %s : processConfig failed: %s", errSave, err)
					}
				}
			}(config)
		}
		return nil
	}
}

//saveErrorMessage saves error message to config
//returns error if not successful, nil otherwise
func saveErrorMessage(config *pb.Config, c pb.ContextBoxServiceClient, err error) error {
	config.CurrentState = config.DesiredState // Update currentState, so we can use it for deletion later
	config.ErrorMessage = err.Error()
	errSave := cbox.SaveConfigScheduler(c, &pb.SaveConfigRequest{Config: config})
	if errSave != nil {
		return fmt.Errorf("error while saving the config: %v", err)
	}
	return nil
}

//healthCheck function is used for querying readiness of the pod running this microservice
func healthCheck() error {
	res, err := createDesiredState(nil)
	if res != nil || err == nil {
		return fmt.Errorf("health check function got unexpected result")
	}
	return nil
}

func main() {
	// initialize logger
	utils.InitLog("scheduler")
	// Create connection to Context-box
	log.Info().Msgf("Dial Context-box: %s", envs.ContextBoxURL)
	cc, err := utils.GrpcDialWithInsecure("context-box", envs.ContextBoxURL)
	if err != nil {
		log.Fatal().Err(err)
	}
	defer func() { utils.CloseClientConnection(cc) }()
	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	// Initialize health probes
	healthChecker := healthcheck.NewClientHealthChecker(fmt.Sprint(defaultSchedulerPort), healthCheck)
	healthChecker.StartProbes()

	g, ctx := errgroup.WithContext(context.Background())
	w := worker.NewWorker(ctx, 10*time.Second, configProcessor(c), worker.ErrorLogger)

	// listen for system interrupts to gracefully shut down
	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)
		<-ch
		return errors.New("scheduler interrupt signal")
	})
	// scheduler goroutine
	g.Go(func() error {
		w.Run()
		return nil
	})
	log.Info().Msgf("Stopping Scheduler: %v", g.Wait())
}
