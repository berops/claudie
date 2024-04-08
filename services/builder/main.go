package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/berops/claudie/services/builder/domain/usecases/metrics"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/internal/worker"
	"github.com/berops/claudie/services/builder/adapters/outbound"
	"github.com/berops/claudie/services/builder/domain/usecases"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/errgroup"
)

const (
	defaultBuilderPort    = 50051
	defaultPrometheusPort = "9090"
	healthCheckInterval   = 10 * time.Second
)

func main() {
	utils.InitLog("builder")

	// Init connections.
	cbox := &outbound.ContextBoxConnector{}
	if err := cbox.Connect(); err != nil {
		log.Err(err).Msgf("Failed to connect to Context-box")
		return
	}
	defer cbox.Disconnect()

	tf := &outbound.TerraformerConnector{}
	if err := tf.Connect(); err != nil {
		log.Err(err).Msgf("Failed to connect to Terraformer")
		return
	}
	defer tf.Disconnect()

	ans := &outbound.AnsiblerConnector{}
	if err := ans.Connect(); err != nil {
		log.Err(err).Msgf("Failed to connect to Ansibler")
		return
	}
	defer ans.Disconnect()

	ke := &outbound.KubeElevenConnector{}
	if err := ke.Connect(); err != nil {
		log.Err(err).Msgf("Failed to connect to Kube-eleven")
		return
	}
	defer ke.Disconnect()

	kb := &outbound.KuberConnector{}
	if err := kb.Connect(); err != nil {
		log.Err(err).Msgf("Failed to connect to Kuber")
		return
	}
	defer kb.Disconnect()

	metricsServer := &http.Server{Addr: fmt.Sprintf(":%s", utils.GetEnvDefault("PROMETHEUS_PORT", defaultPrometheusPort))}
	metrics.MustRegisterCounters()

	usecases := &usecases.Usecases{
		ContextBox:  cbox,
		Terraformer: tf,
		Ansibler:    ans,
		KubeEleven:  ke,
		Kuber:       kb,
	}

	hc := healthcheck.NewHealthCheck(&log.Logger, healthCheckInterval, []healthcheck.HealthCheck{
		{
			Ping:        usecases.Terraformer.PerformHealthCheck,
			ServiceName: "terraformer",
		},
		{
			Ping:        usecases.Ansibler.PerformHealthCheck,
			ServiceName: "ansibler",
		},
		{
			Ping:        usecases.KubeEleven.PerformHealthCheck,
			ServiceName: "kube-eleven",
		},
		{
			Ping:        usecases.Kuber.PerformHealthCheck,
			ServiceName: "kuber",
		},
		{
			Ping:        usecases.ContextBox.PerformHealthCheck,
			ServiceName: "contextbox",
		},
	})

	healthcheck.NewClientHealthChecker(fmt.Sprint(defaultBuilderPort), func() error {
		return hc.CheckForFailures()
	}).StartProbes()

	group, ctx := errgroup.WithContext(context.Background())

	// Listen for SIGTERM or context cancellation.
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
			err = errors.New("interrupt signal")
		}

		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Err(err).Msgf("Failed to shutdown metrics server")
		}

		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)
		return err
	})

	group.Go(func() error {
		wg := sync.WaitGroup{}
		allServicesOk := true

		worker.NewWorker(
			ctx,
			5*time.Second,
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

				return usecases.ConfigProcessor(&wg)
			},
			worker.ErrorLogger,
		).Run()

		log.Info().Msg("Builder stopped checking for new configs")
		log.Info().Msgf("Waiting for already started configs to finish processing")
		wg.Wait()
		log.Debug().Msgf("All spawned go-routines finished")

		return nil
	})

	group.Go(func() error {
		http.Handle("/metrics", promhttp.Handler())
		return metricsServer.ListenAndServe()
	})

	log.Info().Msgf("Stopping Builder : %v", group.Wait())
}
