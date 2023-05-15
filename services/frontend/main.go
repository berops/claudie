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

	// Start Kubernetes liveness and readiness probe responders
	healthcheck.NewClientHealthChecker(fmt.Sprint(healthcheckPort),
		func() error {
			return contextBoxConnector.PerformHealthCheck()
		},
	).StartProbes()

	errGroup, errGroupContext := errgroup.WithContext(context.Background())
	usecaseContext, usecaseCancel := context.WithCancel(context.Background())

	usecases := &usecases.Usecases{
		ContextBox:    contextBoxConnector,
		SaveChannel:   make(chan *usecases.RawManifest),
		DeleteChannel: make(chan *usecases.RawManifest),
		Context:       usecaseContext,
	}

	secretWatcher, err := inboundAdapters.NewSecretWatcher(usecases)
	if err != nil {
		usecaseCancel()
		return err
	}

	// Start watching for any input manifests and process them as needed.
	errGroup.Go(func() error {
		log.Info().Msgf("Frontend is ready to process input manifests")
		go usecases.ProcessManifestFiles()

		log.Info().Msgf("Frontend is watching for any new input manifest")
		return secretWatcher.Monitor()
	})

	// Listen for program interruption signals and shut it down gracefully
	errGroup.Go(func() error {
		shutdownSignalChan := make(chan os.Signal, 1)
		signal.Notify(shutdownSignalChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(shutdownSignalChan)
		// Cancel context for usecases functions
		defer usecaseCancel()

		var err error

		select {
		case <-errGroupContext.Done():
			err = errGroupContext.Err()

		case shutdownSignal := <-shutdownSignalChan:
			log.Info().Msgf("Received program shutdown signal %v", shutdownSignal)
			err = errors.New("program interrupt signal")
		}

		// Wait for all the go-routines to finish their work.
		if err := contextBoxConnector.Disconnect(); err != nil {
			log.Error().Msgf("Failed to gracefully shutdown ContextBoxConnector: %v", err)
		}

		return err
	})

	return errGroup.Wait()
}
