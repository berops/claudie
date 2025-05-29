/*
Copyright 2025 berops.com.

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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1betamanifest "github.com/berops/claudie/internal/api/crd/inputmanifest/v1beta1"
	v1alpha1settings "github.com/berops/claudie/internal/api/crd/settings/v1alpha1"
	"github.com/berops/claudie/services/claudie-operator/server/domain/usecases"
	"github.com/go-logr/logr"
)

const (
	// Delays for requeuing each type of event
	// For example: when a new cluster is created
	// first sync of its state will be done after REQUEUE_NEW time,
	// next sync will be done in REQUEUE_IN_PROGRESS
	REQUEUE_NEW         = 20 * time.Second
	REQUEUE_UPDATE      = 20 * time.Second
	REQUEUE_IN_PROGRES  = 10 * time.Second
	REQUEUE_DELETE      = 20 * time.Second
	REQUEUE_AFTER_ERROR = 30 * time.Second
	finalizerName       = "v1beta1.claudie.io/finalizer"
)

// InputManifestReconciler reconciles a InputManifest object
type InputManifestReconciler struct {
	kc       client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Logger   logr.Logger
	*usecases.Usecases
}

// New returns a new controller for InputManifest resource
func New(kclient client.Client,
	scheme *runtime.Scheme,
	logger logr.Logger,
	recorder record.EventRecorder,
	usecase usecases.Usecases,
) *InputManifestReconciler {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1settings.AddToScheme(scheme))
	utilruntime.Must(v1betamanifest.AddToScheme(scheme))

	return &InputManifestReconciler{
		kc:       kclient,
		Scheme:   scheme,
		Recorder: recorder,
		Logger:   logger,
		Usecases: &usecase,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *InputManifestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1betamanifest.InputManifest{}).
		WatchesRawSource(source.Channel(r.Usecases.SaveAutoscalerEvent, &handler.EnqueueRequestForObject{})).
		Complete(r)
}
