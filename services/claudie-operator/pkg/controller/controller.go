package controller

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	v1beta1manifest "github.com/berops/claudie/internal/api/crd/inputmanifest/v1beta1"
	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"
	managerclient "github.com/berops/claudie/services/manager/client"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *InputManifestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)

	inputManifest := &v1beta1manifest.InputManifest{}
	if err := r.kc.Get(ctx, req.NamespacedName, inputManifest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Prepare the manifest.Manifest type
	// Refresh the inputManifest object Secret Reference
	providersSecrets := make([]v1beta1manifest.ProviderWithData, 0, len(inputManifest.Spec.Providers))

	var missingSecrets []string

	for _, p := range inputManifest.Spec.Providers {
		pwd := v1beta1manifest.ProviderWithData{
			ProviderName: p.ProviderName,
			ProviderType: p.ProviderType,
			Templates:    p.Templates,
		}

		key := client.ObjectKey{
			Name:      p.SecretRef.Name,
			Namespace: p.SecretRef.Namespace,
		}

		if err := r.kc.Get(ctx, key, &pwd.Secret); err != nil {
			if apierrors.IsNotFound(err) {
				missingSecrets = append(missingSecrets, fmt.Sprintf("Provider: %s Secret: %s Namespace: %s", p.ProviderName, p.SecretRef.Name, p.SecretRef.Namespace))
			} else {
				// Uknown fatal error.
				return ctrl.Result{}, err
			}
		}

		providersSecrets = append(providersSecrets, pwd)
	}

	if len(missingSecrets) > 0 {
		msg := fmt.Sprintf("the following secrets referenced inside providers were not found: %v", strings.Join(missingSecrets, ", "))

		r.Recorder.Eventf(
			inputManifest,
			nil,
			corev1.EventTypeWarning,
			"SecretNotFound",
			"FetchingSecrets",
			"%s",
			msg,
		)
		log.Error(nil, msg, "reqeueAfter", REQUEUE_AFTER_ERROR)

		inputManifest.SetWatchResourceStatusWithMsg(msg)
		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}

		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, nil
	}

	// Approximate size of the map to 5 nodes per nodepool
	staticNodeSecrets := make(map[string][]v1beta1manifest.StaticNodeWithData, len(inputManifest.Spec.NodePools.Static))
	// Range over static nodepools an get secret for each static node
	for _, s := range inputManifest.Spec.NodePools.Static {
		nodes := make([]v1beta1manifest.StaticNodeWithData, 0, len(s.Nodes))
		for _, n := range s.Nodes {
			var snwd v1beta1manifest.StaticNodeWithData
			if err := r.kc.Get(ctx, client.ObjectKey{Name: n.SecretRef.Name, Namespace: n.SecretRef.Namespace}, &snwd.Secret); err != nil {
				r.Recorder.Eventf(
					inputManifest,
					nil,
					corev1.EventTypeWarning,
					"ProvisioningFailed",
					"FetchingSecrets",
					"%v",
					err,
				)
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
	rawManifest, err := constructInputManifest(*inputManifest, providersSecrets, staticNodeSecrets)
	if err != nil {
		log.Error(err, "error while using referenced secrets", "will try again in", REQUEUE_AFTER_ERROR)
		r.Recorder.Eventf(
			inputManifest,
			nil,
			corev1.EventTypeWarning,
			"ProvisioningFailed",
			"FetchingSecrets",
			"%v",
			err,
		)
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
		r.Recorder.Eventf(
			inputManifest,
			nil,
			corev1.EventTypeWarning,
			"ProvisioningFailed",
			"ValidatingInputManifest",
			"%v",
			err,
		)
		inputManifest.SetUpdateResourceStatus(v1beta1manifest.InputManifestStatus{
			State: v1beta1manifest.STATUS_ERROR,
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
	currentState := v1beta1manifest.InputManifestStatus{Clusters: make(map[string]v1beta1manifest.ClustersStatus)}

	var (
		// whether the config is currently marked for
		// deletion.
		markedForDeletion bool

		// Whether the config is/was deleted.
		deleted bool

		// Last applied checksum of the config.
		lastChecksum []byte

		// Whether the config already exists.
		alreadyExists bool

		// Current state of the config.
		configState spec.Manifest_State
	)

	for _, config := range resp.Config {
		if inputManifest.GetNamespacedNameDashed() != config.Name {
			continue
		}

		markedForDeletion = config.Manifest.Raw == "" && len(config.Manifest.Checksum) == 0

		configState = config.Manifest.State
		lastChecksum = config.Manifest.Checksum
		alreadyExists = true

		var deletedCount int
		for cluster, state := range config.Clusters {
			if state.Current == nil {
				deletedCount++
			}

			stage := "None"
			if state.InFlight != nil && len(state.InFlight.Pipeline) > 0 {
				current := state.InFlight.Pipeline[state.InFlight.CurrentStage]
				switch current.StageKind.(type) {
				case *spec.Stage_Ansibler:
					stage = "Ansibler"
				case *spec.Stage_KubeEleven:
					stage = "KubeEleven"
				case *spec.Stage_Kuber:
					stage = "Kuber"
				case *spec.Stage_Terraformer:
					stage = "Terraformer"
				default:
					stage = "Unknown"
				}
			}

			status := v1beta1manifest.ClustersStatus{
				State:    state.State.GetStatus().String(),
				Phase:    stage,
				Message:  state.State.GetDescription(),
				Previous: make([]v1beta1manifest.FinishedWorkflow, 0, 1),
			}

			for _, p := range state.State.Previous {
				fw := v1beta1manifest.FinishedWorkflow{
					Status:          p.Status.String(),
					Stage:           p.Stage,
					TaskDescription: p.TaskDescription,
					Timestamp:       "",
				}

				if p.Timestamp != nil {
					fw.Timestamp = p.Timestamp.AsTime().UTC().Format(time.RFC3339)
				}

				status.Previous = append(status.Previous, fw)
			}

			currentState.Clusters[cluster] = status
		}
		deleted = deletedCount == len(config.Clusters)

		break
	}

	// Check if input-manifest has been updated
	// Calculate the manifest checksum in inputManifest resource and
	// compare it against msChecksum in database
	inputManifestMarshalled, err := yaml.Marshal(rawManifest)
	if err != nil {
		return ctrl.Result{RequeueAfter: REQUEUE_AFTER_ERROR}, err
	}
	inputManifestChecksum := hash.Digest(string(inputManifestMarshalled))
	inputManifestUpdated := !bytes.Equal(inputManifestChecksum, lastChecksum)

	switch configState {
	case spec.Manifest_Pending:
		currentState.State = v1beta1manifest.STATUS_WATCH
	case spec.Manifest_Scheduled:
		currentState.State = v1beta1manifest.STATUS_IN_PROGRESS
	case spec.Manifest_Done:
		currentState.State = v1beta1manifest.STATUS_DONE
	case spec.Manifest_Error:
		currentState.State = v1beta1manifest.STATUS_ERROR
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
		if !alreadyExists {
			log.Info("Config has been destroyed. Removing finalizer.", "status", currentState.State)

			controllerutil.RemoveFinalizer(inputManifest, finalizerName)
			if err := r.kc.Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed removing finalizer: %w", err)
			}

			return ctrl.Result{}, nil
		}

		inputManifest.SetUpdateResourceStatus(currentState)

		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}

		for cluster, wf := range currentState.Clusters {
			log.Info("Refreshing state", "cluster", cluster, "stage", wf.Phase, "status", wf.State)
		}

		if controllerutil.ContainsFinalizer(inputManifest, finalizerName) {
			// Prevent calling deleteConfig repeatedly
			if markedForDeletion || deleted {
				return ctrl.Result{RequeueAfter: REQUEUE_WATCH}, nil
			}

			// schedule deletion of manifest
			log.Info("Calling delete config")

			if err := r.DeleteConfig(ctx, rawManifest.Name); err != nil {
				return ctrl.Result{}, err
			}

			inputManifest.SetDeletingStatus()
			if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
			}

			return ctrl.Result{RequeueAfter: REQUEUE_IN_PROGRES}, nil
		}
		return ctrl.Result{RequeueAfter: REQUEUE_IN_PROGRES}, nil
	}

	if !alreadyExists {
		log.Info("Calling create config")

		if err := r.CreateConfig(ctx, &rawManifest, inputManifest.Name, inputManifest.Namespace); err != nil {
			return ctrl.Result{}, err
		}

		inputManifest.SetWatchResourceStatus()
		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed executing finalizer: %w", err)
		}

		return ctrl.Result{RequeueAfter: REQUEUE_WATCH}, nil
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

	if configState == spec.Manifest_Error {
		inputManifest.SetUpdateResourceStatus(currentState)

		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}

		r.Recorder.
			Eventf(
				inputManifest,
				nil,
				corev1.EventTypeWarning,
				"ProvisioningFailed",
				"WorkflowFailed",
				"%v",
				buildProvisioningError(currentState),
			)

		log.Error(buildProvisioningError(currentState), "Error while building")

		// fallthrough here, to allow updating, if any.
	}

	// Update logic if the input manifest has been updated
	// only when the resource is not scheduled for deletion
	if inputManifestUpdated && inputManifest.DeletionTimestamp.IsZero() {
		log.Info("Updating InputManifest", "status", currentState.State)

		if err := r.CreateConfig(ctx, &rawManifest, inputManifest.Name, inputManifest.Namespace); err != nil {
			return ctrl.Result{}, err
		}

		inputManifest.SetUpdateResourceStatus(currentState)

		if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
		}

		return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
	}

	if configState == spec.Manifest_Done {
		log.Info("Build completed", "status", currentState.State)

		// fallthrough here, to re-queue the input manifest for reconciliation again.
	}

	inputManifest.SetUpdateResourceStatus(currentState)
	if err := r.kc.Status().Update(ctx, inputManifest); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed updating status: %w", err)
	}

	return ctrl.Result{RequeueAfter: REQUEUE_UPDATE}, nil
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
