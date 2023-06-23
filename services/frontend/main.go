package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/services/frontend/adapters/inbound/grpc"
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
	// lookup environment variables
	portStr = utils.GetEnvDefault("WEBHOOK_TLS_PORT", "9443")
	certDir = utils.GetEnvDefault("WEBHOOK_CERT_DIR", "./tls")
	webhookPath = utils.GetEnvDefault("WEBHOOK_PATH", "/validate-manifest")

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

	autoscalerChan := make(chan event.GenericEvent)
	usecaseContext, usecaseCancel := context.WithCancel(context.Background())
	usecases := &usecases.Usecases{
		ContextBox:          contextBoxConnector,
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

		// Disconnect from context-box
		if err := contextBoxConnector.Disconnect(); err != nil {
			log.Err(err).Msgf("Failed to gracefully shutdown ContextBoxConnector")
		}
		// Cancel context for usecases functions to terminate manager.
		defer usecaseCancel()
		defer signal.Stop(shutdownSignalChan)
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

	// go func() {
	// 	for {
	// 		time.Sleep(time.Second * 1)
	// 		fmt.Println("Send test event to reconsiler")
	// 		im := v1beta1.InputManifest{}
	// 		im.SetName("testResource")
	// 		im.SetNamespace("default")
	// 		ch <- event.GenericEvent{Object: &im}
	// 	}
	// }()

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

	// Register a healthcheck and readiness endpoint, with path /livez and /healthz
	// https://github.com/kubernetes-sigs/controller-runtime/issues/2127
	if err := mgr.AddHealthzCheck("live", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("ready", healthz.Ping); err != nil {
		return err
	}

	// Starting manager with inputManifest controller and validation webhook
	if err := mgr.Start(usecaseContext); err != nil {
		return err
	}

	return fmt.Errorf("program interrupt signal")
}
