package inboundAdapters

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"claudie/microservices/frontend/domain/usecases"
)

type K8sNotificationsReceiver struct {

	usecases *usecases.Usecases

	// manifestDir represents the path to the directory containing manifest files which will be
	// watched by the k8s sidecar container.
	manifestDir string

	// server is the underlying HTTP server which receives notifications
	// from the k8s sidecar container when the manifest files are created / updated /deleted.
	server *http.Server

	// waitGroup is used to handle a graceful shutdown of the HTTP server.
	// It will wait for all spawned go-routines to finish their work.
	waitGroup sync.WaitGroup
}

func(k *K8sNotificationsReceiver) registerNotificationHandlers( ) {

	var router *http.ServeMux
		router.HandleFunc("/process-manifest-files", k.processManifestFilesHandler)

	k.server.Handler= router
}

// processManifestFilesHandler handles incoming notifications from k8s-sidecar about changes
// (CREATE, UPDATE, DELETE) regarding manifest files in the specified directory.
func(k *K8sNotificationsReceiver) processManifestFilesHandler(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(responseWriter, "HTTP method not allowed", http.StatusMethodNotAllowed)

		return
	}

	k.waitGroup.Add(1)
	go func( ) {
		defer k.waitGroup.Done( )

		k.usecases.ProcessManifestFiles(k.manifestDir)
	}( )
}

func NewK8sNotificationsReceiver(usecases *usecases.Usecases) (*K8sNotificationsReceiver, error) {

	k8sNotificationsReceiver := &K8sNotificationsReceiver{
		usecases: usecases,

		manifestDir: os.Getenv("MANIFEST_DIR"),

		server: &http.Server{ ReadHeaderTimeout: 2 * time.Second },
	}

	k8sNotificationsReceiver.registerNotificationHandlers( )

	return k8sNotificationsReceiver, k8sNotificationsReceiver.PerformHealthCheck( )
}

func(k *K8sNotificationsReceiver) Start(host string, port int) error {
	k.server.Addr= net.JoinHostPort(host, fmt.Sprint(port))

	return k.server.ListenAndServe( )
}

func(k *K8sNotificationsReceiver) PerformHealthCheck( ) error {
	if _, err := os.Stat(k.manifestDir); os.IsNotExist(err) {
		return fmt.Errorf("Manifest directory %v doesn't exist: %w", k.manifestDir, err)}

	return nil
}

func(k *K8sNotificationsReceiver) Stop( ) error {

	// First shutdown the http server to block any incoming connections.
	if err := k.server.Shutdown(context.Background( )); err != nil {
		return err}

	// Wait for all go-routines to finish their work.
	k.waitGroup.Wait( )

	return nil
}