package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/metrics"
	"github.com/berops/claudie/internal/worker"
	"github.com/berops/claudie/services/managerv2/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/errgroup"

	"google.golang.org/grpc"
)

var PrometheusPort = envs.GetOrDefaultInt("PROMETHEUS_PORT", 9090)

func main() {
	loggerutils.Init(service.DurableName)
	if err := run(); err != nil {
		log.Fatal().Msgf("manager service finished with: %s", err)
	}
}

func run() error {
	metricsServer := &http.Server{Addr: fmt.Sprintf(":%v", PrometheusPort)}

	metrics.MustRegisterCounters()
	service.MustRegisterCounters()

	errGroup, errGroupContext := errgroup.WithContext(context.Background())

	manager, err := service.New(errGroupContext, grpc.UnaryInterceptor(metrics.MetricsMiddleware))
	if err != nil {
		return err
	}

	errGroup.Go(func() error {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-errGroupContext.Done():
				ticker.Stop()
				return nil
			case <-ticker.C:
				manager.PerformHealthCheckAndUpdateStatus()
			}
		}
	})

	errGroup.Go(func() error { return manager.Serve() })

	errGroup.Go(func() error {
		worker.NewWorker(
			errGroupContext,
			service.Tick,
			func() error { return manager.WatchForPendingDocuments(errGroupContext) },
			worker.ErrorLogger,
		).Run()
		log.Info().Msg("Exited worker loop running WatchForPendingDocuments")
		return nil
	})

	errGroup.Go(func() error {
		worker.NewWorker(
			errGroupContext,
			service.Tick,
			func() error { return manager.WatchForScheduledDocuments(errGroupContext) },
			worker.ErrorLogger,
		).Run()
		log.Info().Msgf("Exited worker loop running WatchForScheduledDocuments")
		return nil
	})

	errGroup.Go(func() error {
		worker.NewWorker(
			errGroupContext,
			service.Tick,
			func() error { return manager.WatchForDoneOrErrorDocuments(errGroupContext) },
			worker.ErrorLogger,
		).Run()
		log.Info().Msgf("Exited worker loop running WatchForDoneOrErrorDocuments")
		return nil
	})

	errGroup.Go(func() error {
		ctx, stop := signal.NotifyContext(errGroupContext, syscall.SIGTERM)
		defer stop()

		<-ctx.Done()

		err := errGroupContext.Err()
		if err == nil {
			log.Info().Msgf("Received SIGTERM signal")
			err = errors.New("program interruption signal")
		}

		log.Info().Msgf("Closing metrics server")
		if err := metricsServer.Shutdown(errGroupContext); err != nil {
			log.Err(err).Msgf("Failed to shutdown metrics server")
		}

		log.Info().Msg("Gracefully shutting down manager")
		if err := manager.Stop(); err != nil {
			log.Err(err).Msgf("failed to stop manager service")
		}

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
