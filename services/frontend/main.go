package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/utils"
	outboundAdapters "github.com/berops/claudie/services/frontend/adapters/outbound"
	"github.com/berops/claudie/services/frontend/domain/usecases"
	"github.com/berops/claudie/services/frontend/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

const (
	// healthcheckPort is the port on which Kubernetes readiness and liveness probes send request
	// for performing health checks.
	healthcheckPort = ":50058"
)

var (
	// port is the port number that the server will serve. It will be defaulted to 9443 if unspecified.
	portStr string
	// certDir is the directory that contains the server key and certificate. The server key and certificate.
	certDir string
	// path under which the validation webhook will serve
	webhookPath string
)

func main() {
	utils.InitLog("frontend")

	if err := run(); err != nil {
		log.Fatal().Msg(err.Error())
	}
}

func run() error {
	contextBoxConnector := outboundAdapters.NewContextBoxConnector(envs.ContextBoxURL)
	err := contextBoxConnector.Connect()
	if err != nil {
		return err
	}

	usecaseContext, usecaseCancel := context.WithCancel(context.Background())
	usecases := &usecases.Usecases{
		ContextBox:    contextBoxConnector,
		SaveChannel:   make(chan *manifest.Manifest),
		DeleteChannel: make(chan *manifest.Manifest),
		Context:       usecaseContext,
	}
	
	// Interrupt signal listener
	go func() {
		shutdownSignalChan := make(chan os.Signal, 1)
		signal.Notify(shutdownSignalChan, os.Interrupt, syscall.SIGTERM)
		sig := <-shutdownSignalChan

		log.Info().Msgf("Received program shutdown signal %v", sig)

		// Disconnect from context-box
		if err := contextBoxConnector.Disconnect(); err != nil {
			log.Err(err).Msgf("Failed to gracefully shutdown ContextBoxConnector")
		}
		// Cancel context for usecases functions to terminate manager.
		defer usecaseCancel()
		defer signal.Stop(shutdownSignalChan)
	}()

	// Setup inputManifest controller
	crlog.SetLogger(zerologr.New(&log.Logger))
	logger := crlog.Log
	scheme := runtime.NewScheme()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: healthcheckPort,
		Logger:                 logger,
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
		port,
		certDir,
		webhookPath,
		logger,
	)); err != nil {
		return err
	}

	// Register a healthcheck endpoint
	if err := mgr.AddHealthzCheck("/health", healthz.Ping); err != nil {
		return err
	}

	// Starting manager with inputManifest controller and validation webhook
	if err := mgr.Start(usecaseContext); err != nil {
		return err
	}

	return fmt.Errorf("program interrupt signal")
}

func init() {
	// lookup environment variables
	portStr = utils.GetEnvDefault("WEBHOOK_TLS_PORT", "9443")
	certDir = utils.GetEnvDefault("WEBHOOK_CERT_DIR", "./tls")
	webhookPath = utils.GetEnvDefault("WEBHOOK_PATH", "/input-manifest-validator")

}
