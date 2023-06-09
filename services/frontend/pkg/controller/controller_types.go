/*
Copyright 2023 berops.com.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/berops/claudie/services/frontend/domain/usecases"
	v1beta "github.com/berops/claudie/services/frontend/pkg/api/v1beta1"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog/log"
)

var (
	scheme = runtime.NewScheme()
)

const (
	finalizerName = "v1beta1.claudie.io/finalizer"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1beta.AddToScheme(scheme))

}

type manifestController struct {
	mgr            manager.Manager
	controllerPort int
	ctx            context.Context
}

// InputManifestReconciler reconciles a InputManifest object
type InputManifestReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	usecases.Usecases
}

// SetupWithManager sets up the controller with the Manager.
func (r *InputManifestReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta.InputManifest{}).
		Complete(r)
}

// NewManifestController creates a new instance of an controller-runtime that will validate the secret with input-manifest
// It takes a context.Context as a parameter, and retunrs a *manifestController instance
func NewManifestController(usecase *usecases.Usecases) (*manifestController, error) {
	// setup logging
	crlog.SetLogger(zerologr.New(&log.Logger))
	logger := crlog.Log

	// lookup environment variables
	portString, err := getEnvErr("CONTROLLER_TLS_PORT")
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, err
	}

	port = 9443 // TODO:change this

	// create ManifestController object
	var mc = manifestController{
		controllerPort: port,
		ctx:            usecase.Context,
	}

	// Setup the controler manager for input-manifest
	mc.mgr, err = ctrl.NewManager(config.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: ":8081",
		Logger:                 logger,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to set up manifest-controller: %w", err)
	}

	// Register a healthcheck endpoint
	if err := mc.mgr.AddHealthzCheck("/health", healthz.Ping); err != nil {
		return nil, err
	}

	// Regiser inputManifest controller
	// TODO: change this so channel will have human readable name
	if err = (&InputManifestReconciler{
		Client:   mc.mgr.GetClient(),
		Scheme:   mc.mgr.GetScheme(),
		Recorder: mc.mgr.GetEventRecorderFor("InputManifest"),
		Usecases: *usecase,
	}).SetupWithManager(mc.mgr); err != nil {
		return nil, err
	}
	//+kubebuilder:scaffold:builder

	return &mc, nil
}

// Start starts the registered manifest-controller.
// Returns an error if there is an error starting any controller.
func (mc *manifestController) Start() {
	crlog.SetLogger(zerologr.New(&log.Logger))
	logger := crlog.Log

	// Start manager with webhook
	if err := mc.mgr.Start(mc.ctx); err != nil {
		logger.Error(err, "unable to run manifest-controller")
	}
}
