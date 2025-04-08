package controller

import (
	"bytes"
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb/spec"
	v1beta "github.com/berops/claudie/services/claudie-operator/pkg/api/v1beta1"
	managerclient "github.com/berops/claudie/services/manager/client"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InputManifestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
	inputManifest := &v1beta.InputManifest{}

	// Get the inputManifest resource
	if err := r.kc.Get(ctx, req.NamespacedName, inputManifest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Prepare the manifest.Manifest type
	// Refresh the inputManifest object Secret Reference
	providersSecrets := make([]v1beta.ProviderWithData, 0, len(inputManifest.Spec.Providers))
	// Range over Provider objects and request the secret for each provider
	for _, p := range inputManifest.Spec.Providers {
		pwd := v1beta.ProviderWithData{
			ProviderName: p.ProviderName,
			ProviderType: p.ProviderType,
			Templates:    p.Templates,
		}

		key := client.ObjectKey{
			Name:      p.SecretRef.Name,
			Namespace: p.SecretRef.Namespace,
		}

		if err := r.kc.Get(ctx, key, &pwd.Secret); err != nil {
			r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
			log.Error(err, "secret not found", "will try again in", REQUEUE_AFTER_ERROR, "name", p.SecretRef.Name, "namespace", p.SecretRef.Namespace)
			return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, nil
		}
		providersSecrets = append(providersSecrets, pwd)
	}

	// Approximate size of the map to 5 nodes per nodepool
	staticNodeSecrets := make(map[string][]v1beta.StaticNodeWithData, len(inputManifest.Spec.NodePools.Static))
	// Range over static nodepools an get secret for each static node
	for _, s := range inputManifest.Spec.NodePools.Static {
		nodes := make([]v1beta.StaticNodeWithData, 0, len(s.Nodes))
		for _, n := range s.Nodes {
			var snwd v1beta.StaticNodeWithData
			if err := r.kc.Get(ctx, client.ObjectKey{Name: n.SecretRef.Name, Namespace: n.SecretRef.Namespace}, &snwd.Secret); err != nil {
				r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
				log.Error(err, "secret not found", "will try again in", REQUEUE_AFTER_ERROR, "name", n.SecretRef.Name, "namespace", n.SecretRef.Namespace)
				return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, nil
			}

			snwd.Username = "root"
			if n.Username != "" {
				snwd.Username = n.Username
			}

			snwd.Endpoint = n.Endpoint
			nodes = append(nodes, snwd)
		}
		staticNodeSecrets[s.Name] = nodes
	}

	// Create a raw input manifest of manifest.Manifest and pull the referenced secrets into it
	rawManifest, err := mergeInputManifestWithSecrets(*inputManifest, providersSecrets, staticNodeSecrets)
	if err != nil {
		log.Error(err, "error while using referenced secrets", "will try again in", REQUEUE_AFTER_ERROR)
		r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, nil
	}

	// check if templates are defined for dynamic nodepools if not use defaults
	setDefaultTemplates(&rawManifest)

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

	resp, err := r.Manager.ListConfigs(ctx, new(managerclient.ListConfigRequest))
	if err != nil {
		return ctrl.Result{}, err
	}

	// Build the actual state of inputManifest.
	// Based on this, reconcile loop will decide
	// what to do next.
	currentState := v1beta.InputManifestStatus{Clusters: make(map[string]v1beta.ClustersStatus)}

	var deleted bool
	var previousChecksum []byte
	var configExists bool
	var configState spec.Manifest_State
	var manifestRescheduled bool

	for _, config := range resp.Config {
		if inputManifest.GetNamespacedNameDashed() != config.Name {
			continue
		}
		configExists = true
		configState = config.Manifest.State
		previousChecksum = config.Manifest.Checksum
		manifestRescheduled = !bytes.Equal(config.Manifest.LastAppliedChecksum, config.Manifest.Checksum)

		var currentManifest manifest.Manifest
		if err := yaml.Unmarshal([]byte(config.GetManifest().GetRaw()), &currentManifest); err != nil {
			return ctrl.Result{}, err
		}

		nmap := getDynamicNodepoolsMap(&currentManifest)
		for _, desired := range rawManifest.NodePools.Dynamic {
			if current, exists := nmap[desired.Name]; exists {
				if err := nodepoolImmutabilityCheck(&desired, current); err != nil {
					log.Error(err, "immutability check for dynamic nodepools failed", "will try again in", REQUEUE_AFTER_ERROR)
					// nodepool exists and user changed the immutable specs
					r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", err.Error())
					inputManifest.SetUpdateResourceStatus(v1beta.InputManifestStatus{
						State: v1beta.STATUS_ERROR,
					})
					if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
						return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
					}
					return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, nil
				}
			}
		}

		var deletedCount int
		for cluster, state := range config.Clusters {
			if state.Current == nil && state.Desired == nil {
				deletedCount++
			}
			currentState.Clusters[cluster] = v1beta.ClustersStatus{
				State:   state.State.GetStatus().String(),
				Phase:   state.State.GetStage().String(),
				Message: state.State.GetDescription(),
			}
		}
		deleted = deletedCount == len(config.Clusters)
	}

	// Check if input-manifest has been updated
	// Calculate the manifest checksum in inputManifest resource and
	// compare it against msChecksum in database
	inputManifestMarshalled, err := yaml.Marshal(rawManifest)
	if err != nil {
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, err
	}
	inputManifestChecksum := hash.Digest(string(inputManifestMarshalled))
	inputManifestUpdated := !bytes.Equal(inputManifestChecksum, previousChecksum)

	switch {
	case configState == spec.Manifest_Pending:
		currentState.State = v1beta.STATUS_NEW
	case configState == spec.Manifest_Scheduled || manifestRescheduled:
		currentState.State = v1beta.STATUS_IN_PROGRESS
	case configState == spec.Manifest_Done:
		currentState.State = v1beta.STATUS_DONE
	case configState == spec.Manifest_Error:
		currentState.State = v1beta.STATUS_ERROR
	}

	// DELETE && FINALIZER LOGIC
	// Check if resource isn't scheduled for deletion,
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

		if configState == spec.Manifest_Scheduled {
			inputManifest.SetUpdateResourceStatus(currentState)
			if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			for cluster, wf := range currentState.Clusters {
				log.Info("Refreshing state", "cluster", cluster, "stage", wf.Phase, "status", wf.State)
			}
			return ctrl.Result{RequeueAfter: REQUEUE_IN_PROGRES}, nil
		}

		if controllerutil.ContainsFinalizer(inputManifest, finalizerName) {
			// Prevent calling deleteConfig repeatedly
			if inputManifest.Status.State == v1beta.STATUS_SCHEDULED_FOR_DELETION || deleted {
				return ctrl.Result{RequeueAfter: REQUEUE_NEW}, nil
			}
			// schedule deletion of manifest
			inputManifest.SetDeletingStatus()
			if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}
			log.Info("Calling delete config")
			if err := r.DeleteConfig(ctx, rawManifest.Name); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: REQUEUE_DELETE}, nil
		}
		return ctrl.Result{RequeueAfter: REQUEUE_DELETE}, nil
	}

	provisioning := configState == spec.Manifest_Pending || configState == spec.Manifest_Scheduled
	provisioning = provisioning || manifestRescheduled
	if provisioning {
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
			if err := r.CreateConfig(ctx, &rawManifest, inputManifest.Name, inputManifest.Namespace); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: REQUEUE_NEW}, nil
		}

		inputManifest.SetUpdateResourceStatus(currentState)

		// InputManifest is not provisioning but is in a retry loop, allow updating it.
		waitOnInput := configState == spec.Manifest_Pending
		waitOnInput = waitOnInput && manifestRescheduled
		waitOnInput = waitOnInput && inputManifestUpdated
		waitOnInput = waitOnInput && inputManifest.DeletionTimestamp.IsZero()
		if waitOnInput {
			log.Info("InputManifest has been updated", "status", currentState.State)
			if err := r.CreateConfig(ctx, &rawManifest, inputManifest.Name, inputManifest.Namespace); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
		}

		// PROVISIONING LOGIC
		// Refresh inputManifest.status fields
		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}
		for cluster, wf := range currentState.Clusters {
			log.Info("Refreshing state", "cluster", cluster, "stage", wf.Phase, "status", wf.State)
		}

		return ctrl.Result{RequeueAfter: REQUEUE_IN_PROGRES}, nil
	}

	// ERROR logic
	// Refresh cluster status, message an error and end the reconcile,
	// Continue the workflow, to update/end reconcile loop.
	if configState == spec.Manifest_Error {
		// No updates to the inputManifest, output an error and finish the reconcile
		inputManifest.SetUpdateResourceStatus(currentState)
		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}
		r.Recorder.Event(inputManifest, corev1.EventTypeWarning, "ProvisioningFailed", buildProvisioningError(currentState).Error())
		log.Error(buildProvisioningError(currentState), "Error while building")
	}

	// Update logic if the input manifest has been updated
	// only when the resource is not scheduled for deletion
	if inputManifestUpdated && inputManifest.DeletionTimestamp.IsZero() {
		inputManifest.SetUpdateResourceStatus(currentState)
		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}
		log.Info("InputManifest has been updated", "status", currentState.State)
		if err := r.CreateConfig(ctx, &rawManifest, inputManifest.Name, inputManifest.Namespace); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
	}

	// End of reconcile loop, update cluster status - don't requeue the inputManifest object
	inputManifest.SetUpdateResourceStatus(currentState)
	if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
	}
	log.Info("Build completed", "status", currentState.State)
	return ctrl.Result{}, nil
}

func nodepoolImmutabilityCheck(desired, current *manifest.DynamicNodePool) error {
	if desired.ProviderSpec != current.ProviderSpec {
		return fmt.Errorf("dynamic nodepools are immutable, changing the provider specification for %s is not allowed, only 'count' and autoscaling' fields are allowed to be modified, consider creating a new nodepool", current.Name)
	}

	if desired.ServerType != current.ServerType {
		return fmt.Errorf("dynamic nodepools are immutable, changing the server type for %s is not allowed, only 'count' and autoscaling' fields are allowed to be modified, consider creating a new nodepool", current.Name)
	}

	if desired.Image != current.Image {
		return fmt.Errorf("dynamic nodepools are immutable, changing the image for %s is not allowed, only 'count' and autoscaling' fields are allowed to be modified, consider creating a new nodepool", current.Name)
	}

	storageDiskChanged := desired.StorageDiskSize == nil && current.StorageDiskSize != nil
	storageDiskChanged = storageDiskChanged || (desired.StorageDiskSize != nil && current.StorageDiskSize == nil)
	storageDiskChanged = storageDiskChanged || ((desired.StorageDiskSize != nil && current.StorageDiskSize != nil) && (*desired.StorageDiskSize != *current.StorageDiskSize))
	if storageDiskChanged {
		return fmt.Errorf("dynamic nodepools are immutable, changing the storage disk size for %s is not allowed, only 'count' and autoscaling' fields are allowed to be modified, consider creating a new nodepool", current.Name)
	}

	machineSpecChanged := desired.MachineSpec == nil && current.MachineSpec != nil
	machineSpecChanged = machineSpecChanged || (desired.MachineSpec != nil && current.MachineSpec == nil)
	machineSpecChanged = machineSpecChanged || ((desired.MachineSpec != nil && current.MachineSpec != nil) && (*desired.MachineSpec != *current.MachineSpec))
	if machineSpecChanged {
		return fmt.Errorf("dynamic nodepools are immutable, changing the machine spec for %s is not allowed, only 'count' and autoscaling' fields are allowed to be modified, consider creating a new nodepool", current.Name)
	}

	return nil
}

// getDynamicNodepoolsMap will read manifest from the given config and return map of provider names keyed by dynamic nodepool names
func getDynamicNodepoolsMap(m *manifest.Manifest) map[string]*manifest.DynamicNodePool {
	nmap := make(map[string]*manifest.DynamicNodePool)
	for _, np := range m.NodePools.Dynamic {
		nmap[np.Name] = &np
	}

	return nmap
}

func setDefaultTemplates(m *manifest.Manifest) {
	m.ForEachProvider(func(_, typ string, tmpls **manifest.TemplateRepository) bool {
		defaultRepository(tmpls, typ)
		return true
	})
}

func defaultRepository(r **manifest.TemplateRepository, providerTyp string) {
	const (
		repo = "https://github.com/berops/claudie-config"
		path = "templates/terraformer/"
	)

	if *r == nil {
		*r = &manifest.TemplateRepository{
			Repository: repo,
			Path:       path + providerTyp,
		}
		return
	}
}
