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

	"google.golang.org/grpc/connectivity"
)

const (
	defaultBuilderPort    = 50051
	defaultPrometheusPort = "9090"
)

// healthCheck function is function used for querying readiness of the pod running this microservice
func healthCheck(usecases *usecases.Usecases) func() error {
	type HealthCheck struct {
		timeSinceTerraformFailure  *time.Time
		timeSinceAnsiblerFailure   *time.Time
		timeSinceKubeElevenFailure *time.Time
		timeSinceKuberFailure      *time.Time
		timeSinceContextBoxFailure *time.Time
	}
	updateTimeSinceFailure := func(now *time.Time, t **time.Time, err error) {
		if err == nil {
			*t = nil
			return
		}
		if *t == nil {
			*t = now
		}
	}
	healthCheck := func(check *HealthCheck) {
		now := time.Now()
		updateTimeSinceFailure(&now, &check.timeSinceTerraformFailure, usecases.Terraformer.PerformHealthCheck())
		updateTimeSinceFailure(&now, &check.timeSinceAnsiblerFailure, usecases.Ansibler.PerformHealthCheck())
		updateTimeSinceFailure(&now, &check.timeSinceKubeElevenFailure, usecases.KubeEleven.PerformHealthCheck())
		updateTimeSinceFailure(&now, &check.timeSinceKuberFailure, usecases.Kuber.PerformHealthCheck())
		updateTimeSinceFailure(&now, &check.timeSinceContextBoxFailure, usecases.ContextBox.PerformHealthCheck())
	}
	checkFailure := func(t *time.Time, service string, perr error) error {
		if t != nil && time.Since(*t) >= 4*time.Minute {
			if perr != nil {
				return fmt.Errorf("%w; %s is unhealthy", perr, service)
			}
			return fmt.Errorf("%s is unhealthy", service)
		}
		return perr
	}
	signalFailure := func(checker *HealthCheck) error {
		var err error
		err = checkFailure(checker.timeSinceTerraformFailure, "terraformer", err)
		err = checkFailure(checker.timeSinceAnsiblerFailure, "ansibler", err)
		err = checkFailure(checker.timeSinceKubeElevenFailure, "kube-eleven", err)
		err = checkFailure(checker.timeSinceKuberFailure, "kuber", err)
		err = checkFailure(checker.timeSinceContextBoxFailure, "context-box", err)
		return err
	}

	hc := new(HealthCheck)
	return func() error {
		healthCheck(hc)
		return signalFailure(hc)
	}
}

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

	// Start health probes.
	healthcheck.NewClientHealthChecker(fmt.Sprint(defaultBuilderPort), healthCheck(usecases)).StartProbes()

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
		prevGrpcConnectionState := cbox.Connection.GetState()

		worker.NewWorker(
			ctx,
			5*time.Second,
			func() error {
				if utils.IsConnectionReady(cbox.Connection) == nil {
					if prevGrpcConnectionState != connectivity.Ready {
						log.Info().Msgf("Connection to Context-box is now ready")
					}
					prevGrpcConnectionState = connectivity.Ready
				} else {
					log.Warn().Msgf("Connection to Context-box is not ready yet")
					log.Debug().Msgf("Connection to Context-box is %s, waiting for the service to be reachable", cbox.Connection.GetState().String())

					prevGrpcConnectionState = cbox.Connection.GetState()
					cbox.Connection.Connect() // try connecting to the context-box microservice.

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
