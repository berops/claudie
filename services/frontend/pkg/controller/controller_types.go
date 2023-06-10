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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/berops/claudie/services/frontend/domain/usecases"
	v1beta "github.com/berops/claudie/services/frontend/pkg/api/v1beta1"
	"github.com/go-logr/logr"
)

const (
	// Delays for requeing each type of event
	// For example: when a new cluster is created
	// first sync of its state will be done after REQUEUE_NEW time,
	// next sync will be done in REQUEUE_IN_PROGRES
	REQUEUE_NEW         = 20 * time.Second
	REQUEUE_UPDATE      = 20 * time.Second
	REQUEUE_IN_PROGRES  = 5 * time.Second
	REQUEUE_DELETE      = 20 * time.Second
	REQUEUE_AFTER_ERROR = 60 * time.Second
	finalizerName       = "v1beta1.claudie.io/finalizer"
)

// InputManifestReconciler reconciles a InputManifest object
type InputManifestReconciler struct {
	kc       client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	Logger   logr.Logger
	usecases.Usecases
}

// New returns a new controller for InputManifest resource
func New(kclient client.Client,
	scheme *runtime.Scheme,
	logger logr.Logger,
	recorder record.EventRecorder,
	usecase usecases.Usecases) *InputManifestReconciler {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1beta.AddToScheme(scheme))
	return &InputManifestReconciler{
		kc:       kclient,
		Scheme:   scheme,
		Recorder: recorder,
		Logger:   logger,
		Usecases: usecase,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *InputManifestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta.InputManifest{}).
		Complete(r)
}
