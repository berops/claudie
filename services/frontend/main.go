package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	inboundAdapters "github.com/berops/claudie/services/frontend/adapters/inbound"
	outboundAdapters "github.com/berops/claudie/services/frontend/adapters/outbound"
	"github.com/berops/claudie/services/frontend/domain/usecases"
)

// manifestDir stores manifests that should be sent
// to the context-box to be processed.
var manifestDir = os.Getenv("MANIFEST_DIR")

const (
	// healthcheckPort is the port on which the frontend service
	// listens for health checks.
	healthcheckPort = 50058

	// sidecarPort is the port on which the frontend service
	// listens for notification from the k8s-sidecar service
	// about changes in the manifestDir.
	sidecarPort = 50059
)

const (
	// k8sSidecarNotificationsReceiverPort is the port at which the frontend microservice listens for notifications
	// from the K8s-sidecar service about changes in the directory containing the claudie manifest files.
	k8sSidecarNotificationsReceiverPort = 50059
)

func main() {
	utils.InitLog("frontend")

	if err := run(); err != nil {
		log.Fatal().Msg(err.Error())
	}
}

func run() error {
	contextBoxConnector := outboundAdapters.NewContextBoxConnector(envs.ContextBoxURL)
	err := contextBoxConnector.Connect()
	if err != nil {
		return err
	}

	usecases := &usecases.Usecases{
		ContextBox: contextBoxConnector,
	}

	k8sSidecarNotificationsReceiver, err := inboundAdapters.NewK8sSidecarNotificationsReceiver(usecases)
	if err != nil {
		return err
	}

	waitGroup, waitGroupContext := errgroup.WithContext(context.Background())

	waitGroup.Go(func() error {
		shutdownSignalChan := make(chan os.Signal, 1)
		signal.Notify(shutdownSignalChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(shutdownSignalChan)

		var err error

		select {
		case <-waitGroupContext.Done():
			err = waitGroupContext.Err()

		case shutdownSignal := <-shutdownSignalChan:
			log.Info().Msgf("Received program shutdown signal %v", shutdownSignal)
			err = errors.New("Program interruption signal")
		}

		// First shutdown the HTTP server to block any incoming connections.
		// And wait for all the go-routines to finish their work.
		performGracefulShutdown := func() {
			log.Info().Msg("Gracefully shutting down K8sSidecarNotificationsReceiver and ContextBoxCommunicator")

			if err := k8sSidecarNotificationsReceiver.Stop(); err != nil {
				log.Error().Msgf("Failed to gracefully shutdown K8sSidecarNotificationsReceiver: %w", err)
			}

			if err := contextBoxConnector.Disconnect(); err != nil {
				log.Error().Msgf("Failed to gracefully shutdown ContextBoxConnector: %w", err)
			}

		}
		performGracefulShutdown()

		return err
	})

	return waitGroup.Wait()
}
