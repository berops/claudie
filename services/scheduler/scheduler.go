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

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/connectivity"

	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/internal/worker"
	"github.com/berops/claudie/proto/pb"
	outboundAdapters "github.com/berops/claudie/services/scheduler/adapters/outbound"
	"github.com/berops/claudie/services/scheduler/domain/usecases"
)

const (
	defaultHealthcheckPort    = 50056
	defaultConfigPullInterval = 10
)

func main() {
	utils.InitLog("scheduler")

	contextBoxConnector := &outboundAdapters.ContextBoxConnector{}
	err := contextBoxConnector.Connect()
	if err != nil {
		log.Fatal().Err(err)
	}
	defer contextBoxConnector.Disconnect()

	usecases := &usecases.Usecases{
		ContextBox: contextBoxConnector,
	}

	// Initialize health probes
	healthcheck.NewClientHealthChecker(
		fmt.Sprint(defaultHealthcheckPort),
		func() error { return healthCheck(usecases) },
	).StartProbes()

	errGroup, errGroupCtx := errgroup.WithContext(context.Background())

	// Scheduler goroutine periodically pulling config from the context-box microservice scheduler queue and
	// processing it
	errGroup.Go(func() error {
		client := pb.NewContextBoxServiceClient(contextBoxConnector.Connection)
		prevGrpcConnectionState := contextBoxConnector.Connection.GetState()
		group := sync.WaitGroup{}

		worker.NewWorker(
			errGroupCtx,
			defaultConfigPullInterval*time.Second,
			func() error {
				if contextBoxConnector.Connection.GetState() == connectivity.Ready {
					if prevGrpcConnectionState != connectivity.Ready {
						log.Info().Msgf("Connection to Context-box is now ready")
					}
					prevGrpcConnectionState = connectivity.Ready
				} else {
					log.Warn().Msgf("Connection to Context-box is not ready yet")
					log.Debug().Msgf("Connection to Context-box is %s, waiting for the service to be reachable", contextBoxConnector.Connection.GetState().String())

					prevGrpcConnectionState = contextBoxConnector.Connection.GetState()
					contextBoxConnector.Connection.Connect() // try connecting to the context-box microservice.

					return nil
				}

				// After successfully establishing connection with the context-box microservice
				// Pull a config from scheduler queue of context-box and process (build desired state) that config
				return usecases.ConfigProcessor(client, &group)
			},
			worker.ErrorLogger,
		).Run()

		log.Info().Msg("Scheduler stopped checking for new configs")
		log.Info().Msgf("Waiting for already started configs to finish processing")

		group.Wait()
		log.Debug().Msgf("All spawned go-routines finished")

		return nil
	})

	// Listen for system interrupts to gracefully shut down
	errGroup.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)

		// Wait for either the received signal or
		// check if an error occurred in other
		// go-routines.
		var err error
		select {
		case <-errGroupCtx.Done():
			err = errGroupCtx.Err()
		case sig := <-ch:
			log.Info().Msgf("Received program shutdown signal %v", sig)
			err = errors.New("interrupt signal")
		}

		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)

		return err
	})

	log.Info().Msgf("Stopping Scheduler: %v", errGroup.Wait())
}

// healthCheck function is used for querying readiness of the pod running this microservice
func healthCheck(usecases *usecases.Usecases) error {
	res, err := usecases.CreateDesiredState(nil)
	if res != nil || err == nil {
		return fmt.Errorf("health check function got unexpected result")
	}
	return nil
}
