package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/natsutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/managerv2/internal/store"
	"github.com/nats-io/nats.go"
)

// Tick represents the interval at which each manifest state is checked.
const Tick = 10 * time.Second

func (s *Service) WatchForScheduledDocuments(ctx context.Context) error {
	cfgs, err := s.store.ListConfigs(ctx, &store.ListFilter{ManifestState: []string{manifest.Scheduled.String()}})
	if err != nil {
		return err
	}

	for _, scheduled := range cfgs {
		logger := loggerutils.WithProjectName(scheduled.Name)

		var clustersDone int
		var anyError bool

	clusters:
		for cluster, state := range scheduled.Clusters {
			if state.State.Status == spec.WorkflowV2_DONE.String() {
				clustersDone++
				continue
			}

			if state.State.Status == spec.WorkflowV2_ERROR.String() {
				// We count as done as no further work will be done with
				// this cluster even though the state is in error.
				clustersDone++
				anyError = true
				continue
			}

			event := state.Task
			if event == nil {
				logger.Debug().Msgf("Missing task to be worked on for cluster %q, considering as done", cluster)
				// Task should not be nil here under normal working circumstances.
				// But in case it is nil we consider the current cluster as done
				state.State.Status = spec.WorkflowV2_DONE.String()

				if err := s.store.UpdateConfig(ctx, scheduled); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Debug().Msgf("Failed to move cluster %q with missing state to Done, dirty write", cluster)
						break
					}
					logger.Err(err).Msgf("Failed to move cluster %q with missing state to Done", cluster)
				}

				// We break here as the config version was updated thus subsequent changes
				// will error out anyways. Try again on next run.
				break clusters
			}

			switch state.State.Status {
			case spec.WorkflowV2_WAIT_FOR_PICKUP.String():
				nextStage := event.Pipeline[event.CurrentStage].StageKind
				stage := spec.TaskEventV2_Stage_StageKind(spec.TaskEventV2_Stage_StageKind_value[nextStage])
				request, reply, err := jetstreamFromPipelineStage(stage)
				if err != nil {
					// something unexpected but we don't crash the service just ignore and continue.
					logger.Err(err).Msgf("ignoring event %q for cluster %q", event.Id, cluster)
					continue clusters
				}

				headers := nats.Header{}
				headers.Set(nats.MsgIdHdr, event.Id)        // to catch duplicates.
				headers.Set(natsutils.ReplyToHeader, reply) // to which queue the reponse should be send to.

				msg := nats.Msg{
					Subject: request,
					Header:  headers,
					Data:    event.Task,
				}
				ack, err := s.nats.JetStream().PublishMsg(ctx, &msg)
				if err != nil {
					logger.Err(err).Msgf("failed to publish task for cluster %q", cluster)
					// if we failed to publish the message to the queue, we don't know what
					// exactly happened, but we can try with the next cluster.
					// As the message may have been persisted just the response didn't arrive,
					// on the next iteration of the Scheduled Documents we will republish the
					// message and since we have a pretty generous timeout for catching duplicates
					// we should catch that the message was already persisted. If the duplicates
					// timeout expires in the meantime we will reschedule the same task twice
					// which shouldn't pose an issue as they're idemponent.
					continue clusters
				}
				if ack.Duplicate {
					logger.Warn().Msgf("event %q: %q, for cluster %q was submitted more than once but the duplication was caught", event.Id, event.Description, cluster)
					// if it was a duplicate we didn't managed to update the state in the DB last try, thus continue.
				}

				state.State.Status = spec.WorkflowV2_IN_PROGRESS.String()
				logger.Debug().Msgf("Moving event %q cluster %q state to InProgress", event.Id, cluster)
				if err := s.store.UpdateConfig(ctx, scheduled); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Debug().Msgf("Failed to move event %q cluster %q state to InProgress, dirty write", event.Id, cluster)
						break
					}
					logger.Err(err).Msgf("Failed to move event %q cluster %q state to InProgress", event.Id, cluster)
				}

				logger.Info().Msgf("Moved event %q for cluster %q into the work queue", event.Id, cluster)

				// We break here as the config version was updated thus subsequent changes
				// will error out anyways. On the run next changes will be worked on.
				break clusters
			case spec.WorkflowV2_IN_PROGRESS.String():
				// Do nothing as the task is already in the queue.
			default:
				// This should only trigger if either, the databse entry got changed to
				// an unsupported value, or a new state was introduced.
				logger.Error().Msgf("cluster task %q for cluster %q is in unknown state can't proceed", event.Id, cluster)
				continue clusters
			}
		}

		if clustersDone == len(scheduled.Clusters) {
			var newManifestState manifest.State

			if anyError {
				newManifestState = manifest.Error
				logger.Info().Msgf("One of the clusters failed to build successfully, moving manifest to %q state", manifest.Error.String())
			} else {
				newManifestState = manifest.Done
				logger.Info().Msgf("All of the clusters build successfully, moving manifest to %q state", manifest.Done.String())
			}

			ok, err := manifest.ValidStateTransitionString(scheduled.Manifest.State, newManifestState)
			if err != nil || !ok {
				logger.Err(err).Msgf("Cannot transtition from manifest state %q to %q, skipping", scheduled.Manifest.State, newManifestState)
				continue
			}

			scheduled.Manifest.State = newManifestState.String()
			if err := s.store.UpdateConfig(ctx, scheduled); err != nil {
				if errors.Is(err, store.ErrNotFoundOrDirty) {
					logger.Warn().Msgf("Scheduled Config couldn't be updated due to a Dirty Write")
					continue
				}
				logger.Err(err).Msgf("Failed to update scheduled config, skipping.")
				continue
			}
		}
	}

	return nil
}

func (s *Service) WatchForPendingDocuments(ctx context.Context) error {
	cfgs, err := s.store.ListConfigs(ctx, &store.ListFilter{ManifestState: []string{manifest.Pending.String()}})
	if err != nil {
		return err
	}

	for _, cfg := range cfgs {
		name := cfg.Name
		logger := loggerutils.WithProjectName(name)
		logger.Info().Msgf("Processing Pending Config")

		pending, err := store.ConvertToGRPC(cfg)
		if err != nil {
			logger.Err(err).Msgf("Failed to convert from DB representation to Grpc: %q", name)
			continue
		}

		var desiredState map[string]*spec.ClustersV2
		if err := createDesiredState(pending, &desiredState); err != nil {
			logger.Err(err).Msgf("Failed to create desired state, skipping.")
			continue
		}

		result, err := scheduleTasks(pending, desiredState)
		if err != nil {
			logger.Err(err).Msgf("Failed to create tasks, skipping.")
			continue
		}

		ok, err := manifest.ValidStateTransitionString(pending.Manifest.State.String(), manifest.Scheduled)
		if err != nil || !ok {
			logger.Err(err).Msgf("Cannot transtition from manifest state %q to %q, skipping", pending.Manifest.State, manifest.Scheduled)
			continue
		}

		switch result {
		case NotReady:
			logger.Info().Msgf("manifest is not ready to be scheduled, retrying again later")
		case Reschedule:
			pending.Manifest.State = spec.ManifestV2_Scheduled
			logger.Debug().Msgf("Scheduling for intermediate tasks after which the config will be rescheduled again")
		case FinalRetry, NoReschedule:
			logger.Debug().Msgf("Scheduling for tasks after which the config will not be rescheduled again")
			pending.Manifest.State = spec.ManifestV2_Scheduled
			pending.Manifest.LastAppliedChecksum = pending.Manifest.Checksum
		}

		modified, err := store.ConvertFromGRPC(pending)
		if err != nil {
			logger.Err(err).Msgf("failed to convert from Grpc to DB representation: %q", pending.Name)
			continue
		}

		if err := s.store.UpdateConfig(ctx, modified); err != nil {
			if errors.Is(err, store.ErrNotFoundOrDirty) {
				logger.Warn().Msgf("Pending Config couldn't be updated due to a Dirty Write, another retry will start shortly.")
				continue
			}
			logger.Err(err).Msgf("Failed to update pending config, skipping.")
			continue
		}

		switch result {
		case NotReady:
			// do nothing.
		case Reschedule, FinalRetry, NoReschedule:
			logger.Info().Msgf("Config has been successfully processed and moved to the %q state", manifest.Scheduled.String())
		}
	}

	return nil
}

func (g *Service) WatchForDoneOrErrorDocuments(ctx context.Context) error {
	return nil
}

func jetstreamFromPipelineStage(stage spec.TaskEventV2_Stage_StageKind) (string, string, error) {
	switch stage {
	case spec.TaskEventV2_Stage_ANSIBLER:
		return natsutils.AnsiblerRequests, natsutils.AnsiblerResponse, nil
	case spec.TaskEventV2_Stage_KUBER:
		return natsutils.KuberRequests, natsutils.KuberResponse, nil
	case spec.TaskEventV2_Stage_KUBE_ELEVEN:
		return natsutils.KubeElevenRequest, natsutils.KubeElevenResponse, nil
	case spec.TaskEventV2_Stage_TERRAFORMER:
		return natsutils.TerraformerRequest, natsutils.TerraformerResponse, nil
	case spec.TaskEventV2_Stage_UNKNOWN:
		fallthrough
	default:
		return "", "", fmt.Errorf("no mapping exists for stage %#v", stage)
	}
}
