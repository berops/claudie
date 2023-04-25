package inboundAdapters

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/services/frontend/domain/usecases"
)

// Receives notifications from the k8s-sidecar container running in the same pod
// The notifications are regarding new/updated claudie manifests
type K8sSidecarNotificationsReceiver struct {
	usecases *usecases.Usecases

	// manifestDir represents the path to the directory containing claudie manifest files which will be
	// watched by the k8s-sidecar container.
	manifestDir string

	// server is the underlying HTTP server which receives notifications
	// from the k8s sidecar container when the manifest files are created/updated/deleted.
	server *http.Server

	// waitGroup is used to handle a graceful shutdown of the HTTP server.
	// It will wait for all spawned go-routines to finish their work.
	waitGroup sync.WaitGroup
}

// NewK8sSidecarNotificationsReceiver creates an instance of the K8sSidecarNotificationsReceiver struct.
// It registers the notification handling route to the underlying HTTP server.
// Then performs an initial healthcheck of K8sSidecarNotificationsReceiver.
func NewK8sSidecarNotificationsReceiver(usecases *usecases.Usecases) (*K8sSidecarNotificationsReceiver, error) {
	manifestDir, isEnvFound := os.LookupEnv("MANIFEST_DIR")
	if !isEnvFound {
		return nil, fmt.Errorf("env MANIFES_DIR not found")
	}

	k8sSidecarNotificationsReceiver := &K8sSidecarNotificationsReceiver{

		manifestDir: manifestDir,
		usecases:    usecases,

		server: &http.Server{ReadHeaderTimeout: 2 * time.Second},
	}

	k8sSidecarNotificationsReceiver.registerNotificationHandlers()

	go k8sSidecarNotificationsReceiver.watchConfigs()

	return k8sSidecarNotificationsReceiver, k8sSidecarNotificationsReceiver.PerformHealthCheck()
}

// registerNotificationHandlers registers a router to the underlying HTTP server.
// The router contains a route ("/reload") that handles incoming notifications from the K8s-sidecar.
func (k *K8sSidecarNotificationsReceiver) registerNotificationHandlers() {
	var router *http.ServeMux = http.NewServeMux()

	router.HandleFunc("/reload", k.processManifestFilesHandler)

	k.server.Handler = router
}

func (k *K8sSidecarNotificationsReceiver) watchConfigs() {
	k.waitGroup.Add(1)
	defer k.waitGroup.Done()

	k.usecases.WatchConfigs()
}

// Start receiving notifications sent by the K8s sidecar.
// The underlying HTTP server is started.
func (k *K8sSidecarNotificationsReceiver) Start(host string, port int) error {
	k.server.Addr = net.JoinHostPort(host, fmt.Sprint(port))

	return k.server.ListenAndServe()
}

// PerformHealthCheck performs a healthcheck for K8sSidecarNotificationsReceiver.
// Checks whether the provided manifest directory exists or not.
func (k *K8sSidecarNotificationsReceiver) PerformHealthCheck() error {
	if _, err := os.Stat(k.manifestDir); os.IsNotExist(err) {
		return fmt.Errorf("Manifest directory %v doesn't exist: %w", k.manifestDir, err)
	}

	return nil
}

// Stop stops receiving notifications sent by the K8s sidecar.
// The underlying HTTP server is stopped.
func (k *K8sSidecarNotificationsReceiver) Stop() error {
	close(k.usecases.Done)

	// First shutdown the HTTP server to block any incoming notifications.
	if err := k.server.Shutdown(context.Background()); err != nil {
		return err
	}

	// Wait for all go-routines to finish their work.
	k.waitGroup.Wait()

	return nil
}

// processManifestFilesHandler handles incoming notifications from k8s-sidecar container, about changes
// (CREATE, UPDATE, DELETE) regarding manifest files in the specified directory.
func (k *K8sSidecarNotificationsReceiver) processManifestFilesHandler(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(responseWriter, "HTTP method not allowed", http.StatusMethodNotAllowed)

		return
	}

	log.Debug().Msgf("Received notification about change of manifest files in the directory %s", k.manifestDir)

	k.waitGroup.Add(1)
	go func() {
		defer k.waitGroup.Done()

		if err := k.usecases.ProcessManifestFiles(k.manifestDir); err != nil {
			log.Error().Msgf("Failed processing manifest files from directory %s : %v", k.manifestDir, err)
		}
	}()

	responseWriter.WriteHeader(http.StatusOK)
}
