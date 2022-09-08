package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/Berops/claudie/internal/envs"
	"github.com/Berops/claudie/internal/healthcheck"
	"github.com/Berops/claudie/internal/utils"
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
		signal.Notify(ch, os.Interrupt)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
			log.Info().Msg("frontend interrupt signal")
			return s.Shutdown()
		}
	})

	return group.Wait()
}
