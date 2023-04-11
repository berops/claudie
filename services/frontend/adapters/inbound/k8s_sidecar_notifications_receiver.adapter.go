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
	// from the k8s sidecar container when the manifest files are created / updated /deleted.
	server *http.Server

	// waitGroup is used to handle a graceful shutdown of the HTTP server.
	// It will wait for all spawned go-routines to finish their work.
	waitGroup sync.WaitGroup
}

func NewK8sSidecarNotificationsReceiver(usecases *usecases.Usecases) (*K8sSidecarNotificationsReceiver, error) {

	k8sSidecarNotificationsReceiver := &K8sSidecarNotificationsReceiver{

		manifestDir: os.Getenv("MANIFEST_DIR"),
		usecases:    usecases,

		server: &http.Server{ReadHeaderTimeout: 2 * time.Second},
	}

	k8sSidecarNotificationsReceiver.registerNotificationHandlers()

	return k8sSidecarNotificationsReceiver, k8sSidecarNotificationsReceiver.PerformHealthCheck()
}

func (k *K8sSidecarNotificationsReceiver) registerNotificationHandlers() {

	var router *http.ServeMux = http.NewServeMux()

	// TODO: rename the route "/reload" to "/process-manifest-files"
	router.HandleFunc("/reload", k.processManifestFilesHandler)

	k.server.Handler = router
}

func (k *K8sSidecarNotificationsReceiver) Start(host string, port int) error {
	k.server.Addr = net.JoinHostPort(host, fmt.Sprint(port))

	return k.server.ListenAndServe()
}

func (k *K8sSidecarNotificationsReceiver) PerformHealthCheck() error {
	if _, err := os.Stat(k.manifestDir); os.IsNotExist(err) {
		return fmt.Errorf("Manifest directory %v doesn't exist: %w", k.manifestDir, err)
	}

	return nil
}

func (k *K8sSidecarNotificationsReceiver) Stop() error {

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

		k.usecases.ProcessManifestFiles(k.manifestDir)
	}()

	responseWriter.WriteHeader(http.StatusOK)
}
