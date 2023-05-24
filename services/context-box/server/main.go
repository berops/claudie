package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/internal/worker"
	"github.com/berops/claudie/services/context-box/server/adapters/inbound/grpc"
	outboundAdapters "github.com/berops/claudie/services/context-box/server/adapters/outbound"
	"github.com/berops/claudie/services/context-box/server/domain/usecases"
)

func main() {
	// initialize logger
	utils.InitLog("context-box")

	mongoDBConnector := outboundAdapters.NewMongoDBConnector(envs.DatabaseURL)
	err := mongoDBConnector.Connect()
	if err != nil {
		log.Fatal().Msgf("Unable to connect to MongoDB: %v", err)
	}
	err = mongoDBConnector.Init()
	if err != nil {
		log.Fatal().Msgf("Unable to perform initialization tasks for MongoDB: %v", err)
	}
	log.Info().Msgf("Connected to MongoDB")
	defer mongoDBConnector.Disconnect()

	usecases := &usecases.Usecases{
		DB: mongoDBConnector,
	}

	grpcAdapter := &grpc.GrpcAdapter{}
	grpcAdapter.Init(usecases)

	errGroup, errGroupContext := errgroup.WithContext(context.Background())

	// Server goroutine
	errGroup.Go(func() error {
		return grpcAdapter.Serve()
	})

	// Go routine to check and enqueue configs periodically
	errGroup.Go(func() error {
		worker.NewWorker(errGroupContext, 10*time.Second, usecases.EnqueueConfigs, worker.ErrorLogger).
			Run()

		log.Info().Msg("Exited worker loop running EnqueueConfigs")
		return nil
	})

	// Listen for system interruption signals to gracefully shut down
	errGroup.Go(func() error {
		shutdownSignalChan := make(chan os.Signal, 1)
		signal.Notify(shutdownSignalChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(shutdownSignalChan)

		var err error

		// Wait for either the received signal or
		// check if an error occurred in other go-routines.
		select {
		case <-errGroupContext.Done():
			err = errGroupContext.Err()

		case shutdownSignal := <-shutdownSignalChan:
			log.Info().Msgf("Received program shutdown signal %v", shutdownSignal)
			err = errors.New("program interruption signal")
		}

		// Perform graceful shutdown
		log.Info().Msg("Gracefully shutting down GrpcAdapter")
		grpcAdapter.Stop()
		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)

		return err
	})

	log.Info().Msgf("Stopping context-box microservice: %v", errGroup.Wait())
}
