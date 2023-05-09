package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/utils"
	inboundAdapters "github.com/berops/claudie/services/frontend/adapters/inbound"
	outboundAdapters "github.com/berops/claudie/services/frontend/adapters/outbound"
	"github.com/berops/claudie/services/frontend/domain/usecases"
)

const (
	// healthcheckPort is the port on which Kubernetes readiness and liveness probes send request
	// for performing health checks.
	healthcheckPort = 50058

	// k8sSidecarNotificationsReceiverPort is the port at which the frontend microservice listens for notifications
	// from the k8s-sidecar service about changes in the directory containing the claudie manifest files.
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
		Done:       make(chan struct{}),
	}

	k8sSidecarNotificationsReceiver, err := inboundAdapters.NewK8sSidecarNotificationsReceiver(usecases)
	if err != nil {
		return err
	}

	// Start Kubernetes liveness and readiness probe responders
	healthcheck.NewClientHealthChecker(fmt.Sprint(healthcheckPort),
		func() error {
			err := k8sSidecarNotificationsReceiver.PerformHealthCheck()
			if err != nil {
				return err
			}

			return contextBoxConnector.PerformHealthCheck()
		},
	).StartProbes()

	errGroup, errGroupContext := errgroup.WithContext(context.Background())

	// Start receiving notifications from the k8s-sidecar container
	errGroup.Go(func() error {
		log.Info().Msgf("Listening for notifications from K8s-sidecar at port: %v", k8sSidecarNotificationsReceiverPort)
		return k8sSidecarNotificationsReceiver.Start("0.0.0.0", k8sSidecarNotificationsReceiverPort)
	})

	// Listen for program interruption signals and shut it down gracefully
	errGroup.Go(func() error {
		shutdownSignalChan := make(chan os.Signal, 1)
		signal.Notify(shutdownSignalChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(shutdownSignalChan)

		var err error

		select {
		case <-errGroupContext.Done():
			err = errGroupContext.Err()

		case shutdownSignal := <-shutdownSignalChan:
			log.Info().Msgf("Received program shutdown signal %v", shutdownSignal)
			err = errors.New("Program interruption signal")
		}

		// Performing graceful shutdown.

		// First shutdown the HTTP server to block any incoming connections.
		log.Info().Msg("Gracefully shutting down K8sSidecarNotificationsReceiver and ContextBoxConnector")
		if err := k8sSidecarNotificationsReceiver.Stop(); err != nil {
			log.Error().Err(err).Msgf("Failed to gracefully shutdown K8sSidecarNotificationsReceiver")
		}
		// Wait for all the go-routines to finish their work.
		if err := contextBoxConnector.Disconnect(); err != nil {
			log.Error().Err(err).Msgf("Failed to gracefully shutdown ContextBoxConnector")
		}

		return err
	})

	return errGroup.Wait()
}
