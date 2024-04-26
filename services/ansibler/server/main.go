package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/berops/claudie/internal/utils/metrics"
	"github.com/berops/claudie/services/ansibler/server/domain/usecases"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	grpc2 "google.golang.org/grpc"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/services/ansibler/server/adapters/inbound/grpc"
)

const (
	defaultPrometheusPort = "9090"
)

func main() {
	// Initialize logger
	utils.InitLog("ansibler")

	grpcAdapter := grpc.CreateGrpcAdapter(
		&usecases.Usecases{
			SpawnProcessLimit: make(chan struct{}, usecases.SpawnProcessLimit),
		},
		grpc2.ChainUnaryInterceptor(
			metrics.MetricsMiddleware,
			utils.PeerInfoInterceptor(&log.Logger),
		),
	)

	metricsServer := &http.Server{Addr: fmt.Sprintf(":%s", utils.GetEnvDefault("PROMETHEUS_PORT", defaultPrometheusPort))}
	metrics.MustRegisterCounters()

	errGroup, errGroupContext := errgroup.WithContext(context.Background())
	errGroup.Go(grpcAdapter.Serve)

	// Listen for system interruptions to gracefully shut down
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

	log.Info().Msgf("Stopping ansibler microservice: %v", errGroup.Wait())
}
