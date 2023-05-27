package inboundAdapters

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	wbhk "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog/log"
)

var (
	healthcheckPort = ":8081"
	healthcheckPath = "healthz"
)

type manifestController struct {
	mgr                        manager.Manager
	ctx                        context.Context
	validationWebhookPort      int
	validationWebhookCertDir   string
	validationWebhookPath      string
	validationWebhookNamespace string
}

func NewManifestController(ctx context.Context) (*manifestController, error) {
	// lookup environment variables
	portString, err := lookupEnv("WEBHOOK_TLS_PORT")
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}
	namespace, err := lookupEnv("NAMESPACE")
	if err != nil {
		return nil, err
	}
	certDir, err := lookupEnv("WEBHOOK_CERT_DIR")
	if err != nil {
		return nil, err
	}
	webhookPath, err := lookupEnv("WEBHOOK_PATH")
	if err != nil {
		return nil, err
	}

	// create ManifestController object
	var mc = manifestController{
		ctx:                        ctx,
		validationWebhookPort:      port,
		validationWebhookNamespace: namespace,
		validationWebhookPath:      webhookPath,
		validationWebhookCertDir:   certDir,
	}

	// Setup the controler manager for input-manifest
	mc.mgr, err = ctrl.NewManager(config.GetConfigOrDie(), ctrl.Options{
		Namespace:              mc.validationWebhookNamespace,
		HealthProbeBindAddress: healthcheckPort,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to set up manifest-controller: " + err.Error())
	}

	// Register a healthcheck endpoint
	if err := mc.mgr.AddHealthzCheck(healthcheckPath, healthz.Ping); err != nil {
		return nil, err
	}

	// Register the input-manifest validation endpoint
	hookServer := &wbhk.Server{
		Port:    mc.validationWebhookPort,
		CertDir: mc.validationWebhookCertDir,
	}
	hookServer.Register(mc.validationWebhookPath, admission.WithCustomValidator(&corev1.Secret{}, &secretValidator{}))
	if err := mc.mgr.Add(hookServer); err != nil {
		return nil, err
	}

	return &mc, nil
}

func (mc *manifestController) Start() {

	crlog.SetLogger(zerologr.New(&log.Logger))
	logger := crlog.Log

	// Start manager with webhook
	if err := mc.mgr.Start(mc.ctx); err != nil {
		logger.Error(err, "unable to run manifest-controller")
	}
}

func (mc *manifestController) PerformHealthCheck() error {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1%s/%s",healthcheckPort, healthcheckPath))
	log.Debug().Msgf("health status: %s", resp.Status)
	if err != nil {
		return err
	}
	return nil
}

func lookupEnv(env string) (string, error) {
	value, exists := os.LookupEnv(env)
	if !exists {
		return "", fmt.Errorf("environment variable %s not found", env)
	}
	log.Debug().Msgf("Using %s %s", env, value)

	return value, nil
}
