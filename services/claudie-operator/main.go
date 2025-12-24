package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/services/claudie-operator/pkg/controller"
	"github.com/berops/claudie/services/claudie-operator/server/adapters/inbound/grpc"
	"github.com/berops/claudie/services/claudie-operator/server/domain/usecases"
	managerclient "github.com/berops/claudie/services/managerv2/client"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog/log"
)

const (
	// healthcheckPort is the port on which Kubernetes readiness and liveness probes send request
	// for performing health checks.
	healthcheckPort     = ":50000"
	healthCheckInterval = 10 * time.Second
)

var (
	// portStr is the port number that the server will serve. It will be defaulted to 9443 if unspecified.
	portStr string
	// certDir is the directory that contains the server key and certificate. The server key and certificate.
	certDir string
	// path under which the validation webhook will serve
	webhookPath string
	// namespaceSelector filters namespaces to watch for inputManifest resources
	// takes a string input of the form "namespace1,namespace-2,namespace3"
	namespaceSelector string
	// watchedNamespaces is a list of namespaces to watch
	watchedNamespaces []string
)

func main() {
	// lookup environment variables
	portStr = envs.GetOrDefault("WEBHOOK_TLS_PORT", "9443")
	certDir = envs.GetOrDefault("WEBHOOK_CERT_DIR", "./tls")
	webhookPath = envs.GetOrDefault("WEBHOOK_PATH", "/validate-manifest")
	namespaceSelector = envs.GetOrDefault("CLAUDIE_NAMESPACES", cache.AllNamespaces)
	watchedNamespaces = strings.Split(namespaceSelector, ",")
	loggerutils.Init("claudie-operator")

	if err := run(); err != nil {
		log.Fatal().Msg(err.Error())
	}
}

func run() error {
	manager, err := managerclient.New(&log.Logger)
	if err != nil {
		return err
	}
	defer manager.Close()

	autoscalerChan := make(chan event.GenericEvent)
	usecaseContext, usecaseCancel := context.WithCancel(context.Background())
	usecases := &usecases.Usecases{
		Manager:             manager,
		Context:             usecaseContext,
		SaveAutoscalerEvent: autoscalerChan,
	}

	grpcAdapter := &grpc.GrpcAdapter{}
	grpcAdapter.Init(usecases)

	errGroup, errGroupContext := errgroup.WithContext(context.Background())

	// Server goroutine
	errGroup.Go(func() error {
		return grpcAdapter.Serve()
	})

	// Interrupt signal listener
	errGroup.Go(func() error {
		shutdownSignalChan := make(chan os.Signal, 1)
		signal.Notify(shutdownSignalChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(shutdownSignalChan)

		var err error

		// Wait for either the received signal or
		// check if an error occurred in other go-routines.
		select {
		case <-errGroupContext.Done():
			err = errGroupContext.Err()

		case sig := <-shutdownSignalChan:
			log.Info().Msgf("Received program shutdown signal %v", sig)
			err = errors.New("program interruption signal")
		}

		// Cancel context for usecases functions to terminate manager.
		defer usecaseCancel()
		defer signal.Stop(shutdownSignalChan)
		defer close(autoscalerChan)

		// Perform graceful shutdown
		log.Info().Msg("Gracefully shutting down GrpcAdapter")
		grpcAdapter.Stop()
		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)

		return err
	})

	// Setup inputManifest controller
	crlog.SetLogger(zerologr.New(&log.Logger))
	logger := crlog.Log
	scheme := runtime.NewScheme()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: healthcheckPort,
		Logger:                 logger,
		NewCache: func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
			opts.DefaultNamespaces = make(map[string]cache.Config, len(watchedNamespaces))
			for _, ns := range watchedNamespaces {
				opts.DefaultNamespaces[ns] = cache.Config{}
				log.Debug().Msgf("Watching namespace: %s", ns)
			}
			return cache.New(config, opts)
		},
	})
	if err != nil {
		return err
	}

	// Register inputManifest controller
	if err := controller.New(
		mgr.GetClient(),
		mgr.GetScheme(),
		logger,
		mgr.GetEventRecorderFor("InputManifest"),
		*usecases,
	).SetupWithManager(mgr); err != nil {
		return err
	}

	// convert string from env to int
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return err
	}
	// Register inputManifest validation webhook
	if err := mgr.Add(controller.NewWebhook(
		mgr.GetClient(),
		mgr.GetScheme(),
		port,
		certDir,
		webhookPath,
		logger,
	)); err != nil {
		return err
	}

	hc := healthcheck.NewHealthCheck(&log.Logger, healthCheckInterval, []healthcheck.HealthCheck{{
		Ping:        usecases.Manager.HealthCheck,
		ServiceName: "manager",
	}})

	// Register a healthcheck and readiness endpoint, with path and /healthz
	// https://github.com/kubernetes-sigs/controller-runtime/issues/2127
	if err := mgr.AddHealthzCheck("health", func(req *http.Request) error {
		if err := hc.CheckForFailures(); err != nil {
			return err
		}
		return healthz.Ping(req)
	}); err != nil {
		return err
	}

	// Starting manager with inputManifest controller and validation webhook
	if err := mgr.Start(usecaseContext); err != nil {
		return err
	}

	return fmt.Errorf("program interrupt signal")
}
