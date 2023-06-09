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

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/services/context-box/server/utils"
	v1beta "github.com/berops/claudie/services/frontend/pkg/api/v1beta1"
)

//+kubebuilder:rbac:groups=claudie.io,resources=inputmanifests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=claudie.io,resources=inputmanifests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=claudie.io,resources=inputmanifests/finalizers,verbs=update

// TODO: RBAC
// TODO: Add validation webhook

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InputManifestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
	inputManifest := &v1beta.InputManifest{}

	// Get the inputManifest resource
	if err := r.Get(ctx, req.NamespacedName, inputManifest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Get all configs from context-box
	configs, err := r.Usecases.ContextBox.GetAllConfigs()
	if err != nil {
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, err
	}

	// State of config pulled from the DB
	currentState := v1beta.InputManifestStatus{
		Clusters: make(map[string]v1beta.ClustersStatus),
	}
	var currentMsChecksum []byte

	// Check inputManifest object status
	// Build the current inputManifest Status fields
	for _, config := range configs {
		if config.Name == inputManifest.GetNamespacedName() {
			var curretManifestStatus string
			for cluster, workflow := range config.State {
				statuses := &v1beta.ClustersStatus{
					State:   workflow.GetStatus().String(),
					Phase:   workflow.GetStage().String(),
					Message: workflow.GetDescription(),
				}
				currentState.Clusters[cluster] = *statuses

				if workflow.GetStatus().String() == v1beta.STATUS_IN_PROGRESS {
					curretManifestStatus = v1beta.STATUS_IN_PROGRESS
				}
				// Set status to DONE_WITH_ERROR if at least one cluster has an ERROR and other are in DONE state
				switch workflow.GetStatus().String() {
				case v1beta.STATUS_IN_PROGRESS:
					curretManifestStatus = v1beta.STATUS_IN_PROGRESS
				case v1beta.STATUS_SCHEDULED_FOR_DELETION:
					curretManifestStatus = v1beta.STATUS_SCHEDULED_FOR_DELETION
				case v1beta.STATUS_ERROR:
					switch curretManifestStatus {
					case v1beta.STATUS_DONE:
						curretManifestStatus = v1beta.STATUS_DONE_ERROR
					case v1beta.STATUS_SCHEDULED_FOR_DELETION:
						// do nothing
					case v1beta.STATUS_IN_PROGRESS:
						// do nothing
					default:
						curretManifestStatus = v1beta.STATUS_ERROR
					}
				case v1beta.STATUS_DONE:
					switch curretManifestStatus {
					case v1beta.STATUS_ERROR:
						curretManifestStatus = v1beta.STATUS_DONE_ERROR
					case v1beta.STATUS_SCHEDULED_FOR_DELETION:
						// do nothing
					case v1beta.STATUS_IN_PROGRESS:
						// do nothing
					default:
						curretManifestStatus = v1beta.STATUS_DONE
					}
				}
			}
			currentState.State = curretManifestStatus
			currentMsChecksum = config.MsChecksum
		}
	}

	// Prepare the manifest.Manifest type
	// Refresh the inputManifest object Secret Reference
	providersSecrets := []v1beta.ProviderWithData{}
	// Range over Provider objects and request the secret for each provider
	for _, p := range inputManifest.Spec.Providers {
		var pwd v1beta.ProviderWithData
		pwd.ProviderName = p.ProviderName
		pwd.ProviderType = v1beta.ProviderType(p.ProviderType)
		if err := r.Get(ctx, client.ObjectKey{Name: p.SecretRef.Name, Namespace: p.SecretRef.Namespace}, &pwd.Secret); err != nil {
			r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
			log.Error(err, "secret not found", "name", p.SecretRef.Name, "namespace", p.SecretRef.Namespace)
			return ctrl.Result{}, err
		}
		providersSecrets = append(providersSecrets, pwd)
	}

	// Create a raw input manifest of manifest.Manifest and pull the referenced secrets into it
	rawManifest, err := mergeInputManifestWithSecrets(*inputManifest, providersSecrets)
	if err != nil {
		log.Error(err, "error while using referenced secrets")
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, nil
	}

	if err := validateInputManifest(inputManifest); err != nil {
		log.Error(err, "Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaachuj")
	}

	// With the rawManifest filled with providers credentials,
	// the Manifest.Providers{} struct will be properly validated
	if err := rawManifest.Providers.Validate(); err != nil {
		log.Error(err, "error while validating referenced secrets")
		r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
		inputManifest.SetUpdateResourceStatus(v1beta.InputManifestStatus{
			State: v1beta.STATUS_ERROR,
		})

		if err := r.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, nil
	}

	// DELETE && FINALIZER LOGIC
	// Check if resource isn't schedguled for deletion,
	// when true, add finalizer else run delete logic
	if inputManifest.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(inputManifest, finalizerName) {
			controllerutil.AddFinalizer(inputManifest, finalizerName)
			if err := r.Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed executing finalizer: %w", err)
			}
		}
	} else {
		// Resource schedguled for deletion
		// if STATE == "" -> Cluster has been removed. Remove finalizer
		// if STATE == IN_PROGRESS || SCHEDULED_FOR_DELETION -> Wait for all tasks to be finished
		// other case -> call deleteConfig

		if currentState.State == "" {
			log.Info("Config has been destroyed. Removing finalizer.", "status", currentState.State)
			controllerutil.RemoveFinalizer(inputManifest, finalizerName)
			if err := r.Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed removing finalizer: %w", err)
			}
			return ctrl.Result{}, nil
		}

		if currentState.State == v1beta.STATUS_IN_PROGRESS || currentState.State == v1beta.STATUS_SCHEDULED_FOR_DELETION {
			inputManifest.SetUpdateResourceStatus(currentState)
			if err := r.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			log.Info("Refreshing state", "status", currentState.State)
			return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
		}

		if controllerutil.ContainsFinalizer(inputManifest, finalizerName) {
			// schedgule deletion of manifest
			inputManifest.SetDeletingStatus()
			if err := r.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			log.Info("Calling delete config")
			r.deleteConfig(&rawManifest)
			return ctrl.Result{RequeueAfter: REQUEUE_DELETE}, nil
		}
	}

	// Skip for cluster thath are ready
	if currentState.State != v1beta.STATUS_DONE {
		// CREATE LOGIC
		// Add initial status labels for the resource, Requeue the loop
		if currentState.State == ("") {
			if inputManifest.Status.State == v1beta.STATUS_NEW {
				return ctrl.Result{RequeueAfter: REQUEUE_NEW}, nil
			}
			inputManifest.SetNewReousrceStatus()
			if err := r.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed executing finalizer: %w", err)
			}
			log.Info("Calling create config")
			r.createConfig(&rawManifest)
			return ctrl.Result{RequeueAfter: REQUEUE_NEW}, nil
		}

		// PROVISIONING LOGIC
		// Refresh IN_PROGRESS cluster status, requeue the loop
		if currentState.State == v1beta.STATUS_IN_PROGRESS {
			inputManifest.SetUpdateResourceStatus(currentState)
			if err := r.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			log.Info("Refreshing state", "status", currentState.State)
			return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
		}

		// Error logic
		// Refresh cluster status, message an error and end the reconcile, or
		if currentState.State == v1beta.STATUS_ERROR || currentState.State == v1beta.STATUS_DONE_ERROR {
			// No updates to the inputManifest, output an error and finish the reconcile
			inputManifest.SetUpdateResourceStatus(currentState)
			if err := r.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", buildProvisioningError(currentState).Error())
			log.Error(buildProvisioningError(currentState), "Error while building")
			return ctrl.Result{}, nil
		}
	}

	// Check if input-manifest has been updated
	// Calculate the manifest checksum in inputManifest resource and
	// compare it against msChecksum in database
	inputManifestMarshalled, err := yaml.Marshal(rawManifest)
	if err != nil {
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, err
	}
	inputManifestChecksum := utils.CalculateChecksum(string(inputManifestMarshalled))
	inputManifestUpdated := !(utils.Equal(inputManifestChecksum, currentMsChecksum))

	// Update logic if the input manifest has been updated
	// only when the resource is not scheduled for deletion
	if inputManifestUpdated && inputManifest.DeletionTimestamp.IsZero() {
		log.Info("InputManifest has been updates", "status", currentState.State)
		inputManifest.SetUpdateResourceStatus(currentState)
		if err := r.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}
		r.createConfig(&rawManifest)
		return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
	}

	// End of reconcile loop, update cluster status - dont'requeue the inputManifest object
	log.Info("Build compleate", "status", currentState.State)
	inputManifest.SetUpdateResourceStatus(currentState)
	if err := r.Status().Update(ctx, inputManifest); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *InputManifestReconciler) createConfig(im *manifest.Manifest) error {
	r.Usecases.CreateConfig(im)
	return nil
}

func (r *InputManifestReconciler) deleteConfig(im *manifest.Manifest) error {
	r.Usecases.DeleteConfig(im)
	return nil
}
