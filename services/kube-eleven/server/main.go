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
	"github.com/berops/claudie/services/kube-eleven/server/adapters/inbound/grpc"
	"github.com/berops/claudie/services/kube-eleven/server/domain/usecases"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	grpc2 "google.golang.org/grpc"
)

const (
	defaultPrometheusPort = "9090"
)

func main() {
	// Initialize logger
	loggerutils.Init("kube-eleven")

	usecases := &usecases.Usecases{
		SpawnProcessLimit: semaphore.NewWeighted(usecases.SpawnProcessLimit),
	}
	grpcAdapter := grpc.GrpcAdapter{}
	grpcAdapter.Init(
		usecases,
		grpc2.ChainUnaryInterceptor(
			metrics.MetricsMiddleware,
			grpcutils.PeerInfoInterceptor(&log.Logger),
		),
	)

	metricsServer := &http.Server{Addr: fmt.Sprintf(":%s", envs.GetOrDefault("PROMETHEUS_PORT", defaultPrometheusPort))}
	metrics.MustRegisterCounters()

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

		if err := metricsServer.Shutdown(errGroupContext); err != nil {
			log.Err(err).Msgf("Failed to shutdown metrics server")
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

	errGroup.Go(func() error {
		http.Handle("/metrics", promhttp.Handler())
		return metricsServer.ListenAndServe()
	})

	log.Info().Msgf("Stopping Kube-eleven microservice: %s", errGroup.Wait())
}
