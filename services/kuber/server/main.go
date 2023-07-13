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

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/services/kuber/server/adapters/inbound/grpc"
	"github.com/berops/claudie/services/kuber/server/domain/usecases"
)

func main() {
	// Initialize logger
	utils.InitLog("kuber")

	usecases := &usecases.Usecases{}
	grpcAdapter := grpc.GrpcAdapter{}
	grpcAdapter.Init(usecases)

	errGroup, errGroupContext := errgroup.WithContext(context.Background())

	// Start receiving gRPC requests
	errGroup.Go(grpcAdapter.Serve)

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
			err = errors.New("program interruption signal")
		}

		// Performing graceful shutdown.
		grpcAdapter.Stop()

		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)

		return err
	})

	log.Info().Msgf("Stopping Kuber microservice: %s", errGroup.Wait())
}
