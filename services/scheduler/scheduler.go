package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/internal/worker"
	"github.com/berops/claudie/proto/pb"
	cbox "github.com/berops/claudie/services/context-box/client"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"google.golang.org/grpc/connectivity"
)

const (
	defaultSchedulerPort = 50056
)

// processConfig is function used to carry out task specific to Scheduler
// returns error if not successful, nil otherwise
func processConfig(config *pb.Config, c pb.ContextBoxServiceClient) error {
	//create desired state
	config, err := createDesiredState(config)
	if err != nil {
		return fmt.Errorf("error while creating a desired state: %w", err)
	}

	//save config with new desired state
	err = cbox.SaveConfigScheduler(c, &pb.SaveConfigRequest{Config: config})
	if err != nil {
		return fmt.Errorf("error while saving the config: %w", err)
	}

	return nil
}

// configProcessor will fetch new configs from the supplied connection
// to the context-box service. Each received config will be processed in
// a separate go-routine. If a sync.WaitGroup is supplied it will call
// the Add(1) and then the Done() method on it after the go-routine finishes
// the work, if nil it will be ignored.
func configProcessor(c pb.ContextBoxServiceClient, wg *sync.WaitGroup) error {
	//pull an item from a queue in cbox
	res, err := cbox.GetConfigScheduler(c)
	if err != nil {
		return fmt.Errorf("error while getting Scheduler config: %w", err)
	}

	//process config
	config := res.GetConfig()
	if config == nil {
		return nil
	}

	if wg != nil {
		// we received a non-nil config thus we add a new worker to the wait group.
		wg.Add(1)
	}

	go func() {
		if wg != nil {
			defer wg.Done()
		}

		log.Info().Msgf("Processing config %s ", config.Name)
		if err := processConfig(config, c); err != nil {
			log.Error().Msgf("processConfig() failed: %s", err.Error())
			//save error message to config
			errSave := saveErrorMessage(config, c, err)
			if errSave != nil {
				log.Error().Msgf("Failed to save error to the config: %s : processConfig failed: %s", errSave.Error(), err.Error())
			}
		}
		log.Info().Msgf("Config %s have been successfully processed", config.Name)
	}()

	return nil
}

// saveErrorMessage saves error message to config
// returns error if not successful, nil otherwise
func saveErrorMessage(config *pb.Config, c pb.ContextBoxServiceClient, err error) error {
	config.CurrentState = config.DesiredState // Update currentState, so we can use it for deletion later
	config.ErrorMessage = err.Error()
	errSave := cbox.SaveConfigScheduler(c, &pb.SaveConfigRequest{Config: config})
	if errSave != nil {
		return fmt.Errorf("error while saving the config: %w", err)
	}
	return nil
}

// healthCheck function is used for querying readiness of the pod running this microservice
func healthCheck() error {
	res, err := createDesiredState(nil)
	if res != nil || err == nil {
		return fmt.Errorf("health check function got unexpected result")
	}
	return nil
}

func main() {
	utils.InitLog("scheduler")

	cc, err := utils.GrpcDialWithInsecure("context-box", envs.ContextBoxURL)
	if err != nil {
		log.Fatal().Err(err)
	}
	defer func() { utils.CloseClientConnection(cc) }()

	log.Info().Msgf("Initiated connection Context-box: %s, waiting for connection to be in state: %s", envs.ContextBoxURL, connectivity.Ready)

	// Initialize health probes
	healthChecker := healthcheck.NewClientHealthChecker(fmt.Sprint(defaultSchedulerPort), healthCheck)
	healthChecker.StartProbes()

	g, ctx := errgroup.WithContext(context.Background())

	// listen for system interrupts to gracefully shut down
	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)

		// wait for either the received signal or
		// check if an error occurred in other
		// go-routines.
		var err error
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case sig := <-ch:
			log.Info().Msgf("Received signal %v", sig)
			err = errors.New("scheduler interrupt signal")
		}

		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)

		return err
	})

	// scheduler goroutine
	g.Go(func() error {
		client := pb.NewContextBoxServiceClient(cc)
		prevState := cc.GetState()
		group := sync.WaitGroup{}

		worker.NewWorker(
			ctx,
			10*time.Second,
			func() error {
				if cc.GetState() == connectivity.Ready {
					if prevState != connectivity.Ready {
						log.Info().Msgf("connection to Context-box is %s", cc.GetState().String())
					}
					prevState = connectivity.Ready
				} else {
					log.Warn().Msgf("connection to Context-box is not %s", connectivity.Ready.String())
					log.Debug().Msgf("connection to Context-box is %s, waiting for the service to be reachable", cc.GetState().String())

					prevState = cc.GetState()
					cc.Connect() // try connecting to the service.

					return nil
				}
				return configProcessor(client, &group)
			},
			worker.ErrorLogger,
		).Run()

		log.Info().Msg("Exited worker loop and stopped checking for new configs")
		log.Info().Msgf("Waiting for spawned go-routines to finish processing their work")

		group.Wait()

		log.Info().Msgf("All spawned go-routines finished")

		return nil
	})

	log.Info().Msgf("Stopping Scheduler: %v", g.Wait())
}
