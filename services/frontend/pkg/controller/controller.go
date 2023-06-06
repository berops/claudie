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

// TODO: Write down the reconcile loop with boilerplate func's
// TODO: Change the boilerplate func's to actual calling the processor and watcher
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

	log.Info("start loop")

	// TODO: Test with all app running

	configs, err := r.Usecases.ContextBox.GetAllConfigs()
	if err != nil {
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, err
	}

	// state of config pulled from the DB
	currentState := &v1beta.InputManifestStatus{}
	var currentMsChecksum []byte

	// Check inputManifest object status
	for _, config := range configs {
		if config.Name == inputManifest.GetNamespacedName() {
			for _, workflow := range config.State {
				currentState.Message = workflow.GetDescription()
				currentState.Phase = workflow.GetStage().String()
				currentState.State = workflow.GetStatus().String()
			}
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
			log.Error(err, "secret not found", "name", p.SecretRef.Name, "namespace", p.SecretRef.Namespace)
			return ctrl.Result{}, err
		}
		providersSecrets = append(providersSecrets, pwd)
	}

	// create a raw input manifest of manifest.Manifest and mege the referenced secrets with it
	rawManifest, err := mergeInputManifestWithSecrets(*inputManifest, providersSecrets)
	if err != nil {
		return ctrl.Result{}, err
	}

	// temp, move later to validation webhook
	if err := rawManifest.Validate(); err != nil {
		return ctrl.Result{}, err
	}

	// Check if resource isn't schedguled for deletion
	if inputManifest.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !controllerutil.ContainsFinalizer(inputManifest, finalizerName) {
			controllerutil.AddFinalizer(inputManifest, finalizerName)
			if err := r.Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed executing finalizer: %w", err)
			}
		}
	} else {
		// Resource schedguled for deletion
		// if STATE == "" -> Cluster has been removed. Remove finalizer
		// if STATE == IN_PROGRESS -> Wait for all tasks to be finished
		// other case -> schedgule cluster for deletion
		// if STATE == DONE -> calculate manifestChecksum and compare with the DB msChecksum, if different run Update
		// end loop

		// If the resource has DeletionTimestamp and the State is empty - cluster has been deleted, remove the finalizer
		if currentState.State == "" {
			log.Info("Config has been destroyed. Removing finalizer.")
			controllerutil.RemoveFinalizer(inputManifest, finalizerName)
			if err := r.Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed removing finalizer: %w", err)
			}
			return ctrl.Result{}, nil
		}

		// Finish before schedguling a deletion
		if currentState.State == v1beta.STATUS_IN_PROGRESS {
			inputManifest.SetUpdateResourceStatus(*currentState)
			if err := r.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			log.Info("Refreshing state", "status", v1beta.STATUS_IN_PROGRESS)
			return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
		}

		// check if cluster has been deleted. If not update status and requeue

		// Schedgule new config deletion
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

		return ctrl.Result{}, nil
	}

	// Skip for cluster thath are ready
	if currentState.State != v1beta.STATUS_DONE {

		// New resource logic
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

		// In progresss logic
		// Refresh IN_PROGRESS cluster status, requeue the loop
		if currentState.State == v1beta.STATUS_IN_PROGRESS {
			inputManifest.SetUpdateResourceStatus(*currentState)
			if err := r.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			log.Info("refreshing state", "status", v1beta.STATUS_IN_PROGRESS)
			return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
		}

		// Error logic
		// Refresh cluster status, message an error and end the reconcile
		if currentState.State == v1beta.STATUS_ERROR {
			inputManifest.SetUpdateResourceStatus(*currentState)
			if err := r.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			log.Error(fmt.Errorf(currentState.Message), "error while building cluster")
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
	if !utils.Equal(inputManifestChecksum, currentMsChecksum) {
		log.Info("InputManifest has been updates")
		inputManifest.SetUpdateResourceStatus(*currentState)
		if err := r.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}		
		r.SaveChannel <- &rawManifest
		return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
	}
	
	// End of reconcile loop, update cluster status - dont'requeue the inputManifest object
	log.Info("Build compleate")
	inputManifest.SetUpdateResourceStatus(*currentState)
	if err := r.Status().Update(ctx, inputManifest); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *InputManifestReconciler) createConfig(im *manifest.Manifest) error {
	r.Usecases.SaveChannel <- im
	return nil
}

func (r *InputManifestReconciler) deleteConfig(im *manifest.Manifest) error {
	r.Usecases.DeleteChannel <- im
	return nil
}
