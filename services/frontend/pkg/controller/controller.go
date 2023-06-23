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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InputManifestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
	inputManifest := &v1beta.InputManifest{}

	log.Info(req.NamespacedName.String())
	
	// Get the inputManifest resource
	if err := r.kc.Get(ctx, req.NamespacedName, inputManifest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Prepare the manifest.Manifest type
	// Refresh the inputManifest object Secret Reference
	providersSecrets := []v1beta.ProviderWithData{}
	// Range over Provider objects and request the secret for each provider
	for _, p := range inputManifest.Spec.Providers {
		var pwd v1beta.ProviderWithData
		pwd.ProviderName = p.ProviderName
		pwd.ProviderType = p.ProviderType
		if err := r.kc.Get(ctx, client.ObjectKey{Name: p.SecretRef.Name, Namespace: p.SecretRef.Namespace}, &pwd.Secret); err != nil {
			r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
			log.Error(err, "secret not found", "will try again in", REQUEUE_AFTER_ERROR, "name", p.SecretRef.Name, "namespace", p.SecretRef.Namespace)
			return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, nil
		}
		providersSecrets = append(providersSecrets, pwd)
	}
	
	// Create a raw input manifest of manifest.Manifest and pull the referenced secrets into it
	rawManifest, err := mergeInputManifestWithSecrets(*inputManifest, providersSecrets)
	if err != nil {
		log.Error(err, "error while using referenced secrets", "will try again in", REQUEUE_AFTER_ERROR)
		r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, nil
	}

	// With the rawManifest filled with providers credentials,
	// the Manifest.Providers{} struct will be properly validated
	// In case the validation will fail, it will end the reconcile
	// with an err, and generate an Kubernetes Event
	if err := rawManifest.Providers.Validate(); err != nil {
		log.Error(err, "error while validating referenced secrets", "will try again in", REQUEUE_AFTER_ERROR)
		r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
		inputManifest.SetUpdateResourceStatus(v1beta.InputManifestStatus{
			State: v1beta.STATUS_ERROR,
		})
		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, nil
	}

	// Get all configs from context-box
	configs, err := r.Usecases.ContextBox.GetAllConfigs()
	if err != nil {
		return ctrl.Result{}, err
	}

	// Build the actual state of inputManifet.
	// Based on this, reconcile loop will decide
	// what to do next.
	currentState := v1beta.InputManifestStatus{
		Clusters: make(map[string]v1beta.ClustersStatus),
	}
	var dbDsChecksum []byte
	configExists := false
	configInDesiredState := false
	configContainsError := false

	for _, config := range configs {
		if inputManifest.GetNamespacedNameDashed() == config.Name {
			configExists = true
			dbDsChecksum = config.DsChecksum

			if len(config.CsChecksum) == 0 && len(config.DsChecksum) == 0 {
				configInDesiredState = false // handle newly created resources
			} else {
				configInDesiredState = utils.Equal(config.CsChecksum, config.DsChecksum)
			}
			for cluster, workflow := range config.State {
				currentState.Clusters[cluster] = v1beta.ClustersStatus{
					State:   workflow.GetStatus().String(),
					Phase:   workflow.GetStage().String(),
					Message: workflow.GetDescription(),
				}

				if workflow.GetStatus().String() == v1beta.STATUS_ERROR {
					configContainsError = true
				}
			}
		}
	}

	// Set inputManifest status field - informational only
	if !configExists {
		currentState.State = v1beta.STATUS_NEW
	} else if !configInDesiredState {
		currentState.State = v1beta.STATUS_IN_PROGRESS
	} else if configContainsError {
		// Set manifest state to ERROR, if any DONE cluster will be found
		// set it to DONE_WITH_ERROR
		currentState.State = v1beta.STATUS_ERROR
		for _, cluster := range currentState.Clusters {
			if cluster.State == v1beta.STATUS_DONE {
				currentState.State = v1beta.STATUS_DONE_ERROR
			}
		}
	} else {
		currentState.State = v1beta.STATUS_DONE
	}

	// DELETE && FINALIZER LOGIC
	// Check if resource isn't schedguled for deletion,
	// when true, add finalizer else run delete logic
	if inputManifest.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(inputManifest, finalizerName) {
			controllerutil.AddFinalizer(inputManifest, finalizerName)
			if err := r.kc.Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed executing finalizer: %w", err)
			}
		}
	} else {
		if !configExists {
			log.Info("Config has been destroyed. Removing finalizer.", "status", currentState.State)
			controllerutil.RemoveFinalizer(inputManifest, finalizerName)
			if err := r.kc.Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed removing finalizer: %w", err)
			}
			return ctrl.Result{}, nil
		}

		if !configInDesiredState {
			inputManifest.SetUpdateResourceStatus(currentState)
			if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			log.Info("Refreshing state", "status", currentState.State)
			return ctrl.Result{RequeueAfter: REQUEUE_IN_PROGRES}, nil
		}

		if controllerutil.ContainsFinalizer(inputManifest, finalizerName) {
			// Prevent calling deleteConfig, when the deleteConfig call
			// won't make it yet to scheduler
			if inputManifest.Status.State == v1beta.STATUS_SCHEDULED_FOR_DELETION {
				return ctrl.Result{RequeueAfter: REQUEUE_NEW}, nil
			}
			// schedgule deletion of manifest
			inputManifest.SetDeletingStatus()
			if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			log.Info("Calling delete config")
			if err := r.deleteConfig(&rawManifest); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: REQUEUE_DELETE}, nil
		}
		return ctrl.Result{RequeueAfter: REQUEUE_DELETE}, nil
	}

	// Skip the inputManifests thath are ready
	if !configInDesiredState {
		// CREATE LOGIC
		// Call create config if it not present in DB
		if !configExists {
			// Prevent calling createConfig, when the inputManifest
			// won't make it yet to DB
			if inputManifest.Status.State == v1beta.STATUS_NEW {
				return ctrl.Result{RequeueAfter: REQUEUE_NEW}, nil
			}
			inputManifest.SetNewResourceStatus()
			if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed executing finalizer: %w", err)
			}
			log.Info("Calling create config")
			if err := r.createConfig(&rawManifest); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: REQUEUE_NEW}, nil
		}

		// PROVISIONING LOGIC
		// Refresh inputManifest.status fields
		inputManifest.SetUpdateResourceStatus(currentState)
		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}
		log.Info("Refreshing state", "status", currentState.State)
		return ctrl.Result{RequeueAfter: REQUEUE_IN_PROGRES}, nil
	}

	// ERROR logic
	// Refresh cluster status, message an error and end the reconcile, or
	if configContainsError {
		// No updates to the inputManifest, output an error and finish the reconcile
		inputManifest.SetUpdateResourceStatus(currentState)
		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}
		r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", buildProvisioningError(currentState).Error())
		log.Error(buildProvisioningError(currentState), "Error while building")
		return ctrl.Result{}, nil
	}

	// Check if input-manifest has been updated
	// Calculate the manifest checksum in inputManifest resource and
	// compare it against msChecksum in database
	inputManifestMarshalled, err := yaml.Marshal(rawManifest)
	if err != nil {
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, err
	}
	inputManifestChecksum := utils.CalculateChecksum(string(inputManifestMarshalled))
	inputManifestUpdated := !(utils.Equal(inputManifestChecksum, dbDsChecksum))

	// Update logic if the input manifest has been updated
	// only when the resource is not scheduled for deletion
	if inputManifestUpdated && inputManifest.DeletionTimestamp.IsZero() {
		inputManifest.SetUpdateResourceStatus(currentState)
		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}
		log.Info("InputManifest has been updates", "status", currentState.State)
		if err := r.createConfig(&rawManifest); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
	}

	// End of reconcile loop, update cluster status - dont'requeue the inputManifest object
	inputManifest.SetUpdateResourceStatus(currentState)
	if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
	}
	log.Info("Build compleate", "status", currentState.State)
	return ctrl.Result{}, nil
}

func (r *InputManifestReconciler) createConfig(im *manifest.Manifest) error {
	if err := r.Usecases.CreateConfig(im); err != nil {
		return err
	}
	return nil
}

func (r *InputManifestReconciler) deleteConfig(im *manifest.Manifest) error {
	if err := r.Usecases.DeleteConfig(im); err != nil {
		return err
	}
	return nil
}
