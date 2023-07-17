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

	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/internal/worker"
	"github.com/berops/claudie/services/builder/adapters/outbound"
	"github.com/berops/claudie/services/builder/domain/usecases"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/connectivity"
)

const defaultBuilderPort = 50051

// healthCheck function is function used for querying readiness of the pod running this microservice
func healthCheck(usecases *usecases.Usecases) func() error {
	return func() error {
		if usecases.Terraformer.PerformHealthCheck() != nil {
			return errors.New("terraformer is unhealthy")
		}
		if usecases.Ansibler.PerformHealthCheck() != nil {
			return errors.New("ansibler is unhealthy")
		}
		if usecases.KubeEleven.PerformHealthCheck() != nil {
			return errors.New("kube-eleven is unhealthy")
		}
		if usecases.Kuber.PerformHealthCheck() != nil {
			return errors.New("kuber is unhealthy")
		}
		if usecases.ContextBox.PerformHealthCheck() != nil {
			return errors.New("context-box is unhealthy")
		}
		return nil
	}
}

func main() {
	utils.InitLog("builder")

	// Init connections.
	cbox := &outbound.ContextBoxConnector{}
	if err := cbox.Connect(); err != nil {
		log.Err(err).Msgf("Failed to connect to Context-box")
		return
	}
	defer cbox.Disconnect()

	tf := &outbound.TerraformerConnector{}
	if err := tf.Connect(); err != nil {
		log.Err(err).Msgf("Failed to connect to Terraformer")
		return
	}
	defer tf.Disconnect()

	ans := &outbound.AnsiblerConnector{}
	if err := ans.Connect(); err != nil {
		log.Err(err).Msgf("Failed to connect to Ansibler")
		return
	}
	defer ans.Disconnect()

	ke := &outbound.KubeElevenConnector{}
	if err := ke.Connect(); err != nil {
		log.Err(err).Msgf("Failed to connect to Kube-eleven")
		return
	}
	defer ke.Disconnect()

	kb := &outbound.KuberConnector{}
	if err := kb.Connect(); err != nil {
		log.Err(err).Msgf("Failed to connect to Kuber")
		return
	}
	defer kb.Disconnect()

	usecases := &usecases.Usecases{
		ContextBox:  cbox,
		Terraformer: tf,
		Ansibler:    ans,
		KubeEleven:  ke,
		Kuber:       kb,
	}

	// Start health probes.
	healthcheck.NewClientHealthChecker(fmt.Sprint(defaultBuilderPort), healthCheck(usecases)).StartProbes()

	group, ctx := errgroup.WithContext(context.Background())

	// Listen for SIGTERM or context cancellation.
	group.Go(func() error {
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
			err = errors.New("interrupt signal")
		}

		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)
		return err
	})

	group.Go(func() error {
		wg := sync.WaitGroup{}
		prevGrpcConnectionState := cbox.Connection.GetState()

		worker.NewWorker(
			ctx,
			5*time.Second,
			func() error {
				if utils.IsConnectionReady(cbox.Connection) == nil {
					if prevGrpcConnectionState != connectivity.Ready {
						log.Info().Msgf("Connection to Context-box is now ready")
					}
					prevGrpcConnectionState = connectivity.Ready
				} else {
					log.Warn().Msgf("Connection to Context-box is not ready yet")
					log.Debug().Msgf("Connection to Context-box is %s, waiting for the service to be reachable", cbox.Connection.GetState().String())

					prevGrpcConnectionState = cbox.Connection.GetState()
					cbox.Connection.Connect() // try connecting to the context-box microservice.

					return nil
				}
				return usecases.ConfigProcessor(&wg)
			},
			worker.ErrorLogger,
		).Run()

		log.Info().Msg("Builder stopped checking for new configs")
		log.Info().Msgf("Waiting for already started configs to finish processing")
		wg.Wait()
		log.Debug().Msgf("All spawned go-routines finished")

		return nil
	})
	log.Info().Msgf("Stopping Builder : %v", group.Wait())
}
