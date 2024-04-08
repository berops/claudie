package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/berops/claudie/services/scheduler/domain/usecases/metrics"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/internal/worker"
	"github.com/berops/claudie/proto/pb"
	outboundAdapters "github.com/berops/claudie/services/scheduler/adapters/outbound"
	"github.com/berops/claudie/services/scheduler/domain/usecases"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultHealthcheckPort    = 50056
	defaultConfigPullInterval = 10
	defaultPrometheusPort     = "9090"
	healthCheckInterval       = 10 * time.Second
)

func main() {
	utils.InitLog("scheduler")

	contextBoxConnector := &outboundAdapters.ContextBoxConnector{}
	err := contextBoxConnector.Connect()
	if err != nil {
		log.Fatal().Err(err)
	}
	defer contextBoxConnector.Disconnect()

	usecases := &usecases.Usecases{
		ContextBox: contextBoxConnector,
	}

	metricsServer := &http.Server{Addr: fmt.Sprintf(":%s", utils.GetEnvDefault("PROMETHEUS_PORT", defaultPrometheusPort))}
	metrics.MustRegisterCounters()

	hc := healthcheck.NewHealthCheck(&log.Logger, healthCheckInterval, []healthcheck.HealthCheck{{
		Ping:        usecases.ContextBox.PerformHealthCheck,
		ServiceName: "contextbox",
	}})

	healthcheck.NewClientHealthChecker(fmt.Sprint(defaultHealthcheckPort), func() error {
		return hc.CheckForFailures()
	}).StartProbes()

	errGroup, errGroupCtx := errgroup.WithContext(context.Background())

	// Scheduler goroutine periodically pulling config from the context-box microservice scheduler queue and
	// processing it
	errGroup.Go(func() error {
		client := pb.NewContextBoxServiceClient(contextBoxConnector.Connection)
		group := sync.WaitGroup{}
		allServicesOk := true

		worker.NewWorker(
			errGroupCtx,
			defaultConfigPullInterval*time.Second,
			func() error {
				if healthIssues := hc.AnyServiceUnhealthy(); !healthIssues {
					if !allServicesOk {
						log.Info().Msgf("All dependent services are now healthy")
					}
					allServicesOk = true
				} else {
					if allServicesOk {
						log.Warn().Msgf("Waiting for all dependent services to be healthy")
					}
					allServicesOk = false
					return nil
				}
				return usecases.ConfigProcessor(client, &group)
			},
			worker.ErrorLogger,
		).Run()

		log.Info().Msg("Scheduler stopped checking for new configs")
		log.Info().Msgf("Waiting for already started configs to finish processing")

		group.Wait()
		log.Debug().Msgf("All spawned go-routines finished")

		return nil
	})

	// Listen for system interrupts to gracefully shut down
	errGroup.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)

		// Wait for either the received signal or
		// check if an error occurred in other
		// go-routines.
		var err error
		select {
		case <-errGroupCtx.Done():
			err = errGroupCtx.Err()
		case sig := <-ch:
			log.Info().Msgf("Received program shutdown signal %v", sig)
			err = errors.New("interrupt signal")
		}

		if err := metricsServer.Shutdown(errGroupCtx); err != nil {
			log.Err(err).Msgf("Failed to shutdown metrics server")
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

	log.Info().Msgf("Stopping Scheduler: %v", errGroup.Wait())
}

// healthCheck function is function used for querying readiness of the pod running this microservice
func healthCheck(usecases *usecases.Usecases) func() error {
	return func() error {
		if usecases.ContextBox.PerformHealthCheck() != nil {
			return errors.New("context-box is unhealthy")
		}
		return nil
	}
}
