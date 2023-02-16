package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/utils"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/errgroup"
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

func main() {
	utils.InitLog("frontend")

	if err := run(); err != nil {
		log.Fatal().Msg(err.Error())
	}
}

func run() error {
	s, err := newServer(manifestDir, envs.ContextBoxURL)
	if err != nil {
		return err
	}

	healthcheck.NewClientHealthChecker(
		fmt.Sprint(healthcheckPort),
		s.healthcheck(),
	).StartProbes()

	group, ctx := errgroup.WithContext(context.Background())

	group.Go(func() error {
		log.Info().Msgf("Listening for notifications from k8s-sidecar on port: %v", sidecarPort)
		return s.ListenAndServe("0.0.0.0", sidecarPort)
	})

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
			err = errors.New("frontend interrupt signal")
		}

		log.Info().Msg("Gracefully shutting down server")
		if serverErr := s.GracefulShutdown(); serverErr != nil {
			err = fmt.Errorf("%w: failed to gracefully shutdown server", err)
		}

		return err
	})

	return group.Wait()
}
