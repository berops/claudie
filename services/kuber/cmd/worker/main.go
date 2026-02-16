package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/metrics"
	"github.com/berops/claudie/services/kuber/internal/worker/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/errgroup"

	"google.golang.org/grpc"
)

var (
	PrometheusPort = envs.GetOrDefaultInt("PROMETHEUS_PORT", 9090)
)

func main() {
	loggerutils.Init(service.DurableName)

	if err := run(); err != nil {
		log.Fatal().Msgf("ansibler service finished with: %s", err)
	}
}

func run() error {
	metricsServer := &http.Server{
		Addr: fmt.Sprintf(":%v", PrometheusPort),
	}
	metrics.MustRegisterCounters()

	errGroup, errGroupCtx := errgroup.WithContext(context.Background())

	kuber, err := service.New(
		errGroupCtx,
		grpc.ChainUnaryInterceptor(
			metrics.MetricsMiddleware,
			grpcutils.PeerInfoInterceptor(&log.Logger),
		),
	)
	if err != nil {
		return err
	}

	errGroup.Go(kuber.ServeHealthChecks)

	// Check if service is in ready state every 30s
	errGroup.Go(func() error {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-errGroupCtx.Done():
				ticker.Stop()
				return nil
			case <-ticker.C:
				kuber.PerformHealthCheckAndUpdateStatus()
			}
		}
	})

	// Listen for system interruptions to gracefully shut down
	// Listen for program interruption signals and shut it down gracefully
	errGroup.Go(func() error {
		shutdownSignalChan := make(chan os.Signal, 1)
		signal.Notify(shutdownSignalChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(shutdownSignalChan)

		var err error

		select {
		case <-errGroupCtx.Done():
			err = errGroupCtx.Err()

		case shutdownSignal := <-shutdownSignalChan:
			log.Info().Msgf("Received program shutdown signal %v", shutdownSignal)
			err = errors.New("program interruption signal")
		}

		if err := metricsServer.Shutdown(errGroupCtx); err != nil {
			log.Err(err).Msgf("Failed to shutdown metrics server")
		}

		kuber.Stop()

		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)

		return err
	})

	errGroup.Go(func() error {
		http.Handle("/metrics", promhttp.Handler())
		return metricsServer.ListenAndServe()
	})

	return errGroup.Wait()
}
