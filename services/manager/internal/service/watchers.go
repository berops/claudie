package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/natsutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/service/metrics"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/nats-io/nats.go"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	// PendingTick represents the interval at which each manifest state is checked
	// while in the [manifest.Pending] state.
	PendingTick = 12 * time.Second

	// Tick represents the interval at which each manifest state is checked while
	// in the [manifest.Pending] state.
	Tick = 1 * time.Second
)

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
			if state.State.Status == spec.Workflow_DONE.String() {
				clustersDone++
				continue clusters
			}

			if state.State.Status == spec.Workflow_ERROR.String() {
				// We count as done as no further work will be done with
				// this cluster even though the state is in error.
				clustersDone++
				anyError = true
				continue clusters
			}

			event := state.InFlight

			noWork := event == nil
			noWork = noWork || len(event.Pipeline) < 1
			noWork = noWork || (int(event.CurrentStage) > len(event.Pipeline))
			if noWork {
				logger.Debug().Msgf("Nothing to be worked on for cluster %q, considering as done", cluster)

				state.InFlight = nil
				state.State.Status = spec.Workflow_DONE.String()
				state.State.Description = ""

				if err := s.store.UpdateConfig(ctx, scheduled); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Debug().Msgf("Failed to move cluster %q with missing state to Done, dirty write", cluster)
						break clusters
					}
					logger.Err(err).Msgf("Failed to move cluster %q with missing state to Done", cluster)
					continue clusters
				}

				// We break here as the config version was updated thus subsequent changes
				// will error out anyways. Try again on next run.
				break clusters
			}

			pipeline, err := store.ConvertToGRPCStages(event.Pipeline)
			if err != nil {
				logger.Err(err).Msgf("Failed to unmarshal the pipeline for task %q from cluster %q", event.Id, cluster)
				continue clusters
			}

			switch state.State.Status {
			case spec.Workflow_WAIT_FOR_PICKUP.String():
				msg, msgDescription, err := messageForStage(
					scheduled.Name,
					cluster,
					event.Id,
					event.Task,
					pipeline[event.CurrentStage],
				)
				if err != nil {
					// unexpected but we don't crash the service just ignore and continue.
					logger.Err(err).Msgf("ignoring event %q for cluster %q", event.Id, cluster)
					continue clusters
				}

				logger.Debug().Msgf("Publishing msg %q to subject %q", event.Id, msg.Subject)

				ack, err := s.nts.client.JetStream().PublishMsg(ctx, &msg)
				if err != nil {
					logger.Err(err).Msgf("failed to publish task for cluster %q", cluster)
					// failed to publish the message to the queue, unsure what
					// exactly happened, but we can try with the next cluster.
					//
					// As the message may have been persisted just the response didn't arrive,
					// on the next iteration of the Scheduled Documents we will republish the
					// message and since we have a pretty generous timeout for catching duplicates
					// we should catch that the message was already persisted. If the duplicates
					// timeout expires in the meantime we will reschedule the same task twice
					// which shouldn't pose an issue as they're idemponent.
					continue clusters
				}

				if ack.Duplicate {
					metrics.NatsDuplicateMessagesCounter.Inc()

					// Each message for a specific stage has its own ID for catching duplicates
					// The pattern is [TaskID]-[StageName], thus it's not possible to catch duplicate
					// messages from another stage here and there had to be some network issues.
					logger.
						Warn().
						Msgf("event %q: %q, for cluster %q was submitted more than once but the duplication was caught, will move the task to the next stage, assuming the last try failed", event.Id, event.Description, cluster)
				}

				state.State.Status = spec.Workflow_IN_PROGRESS.String()
				state.State.Description = fmt.Sprintf("%s\n- %s", event.Description, msgDescription)

				logger.Debug().Msgf("Moving event %q cluster %q state to InProgress", event.Id, cluster)
				if err := s.store.UpdateConfig(ctx, scheduled); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Debug().Msgf("Failed to move event %q cluster %q state to InProgress, dirty write", event.Id, cluster)
						break
					}
					logger.Err(err).Msgf("Failed to move event %q cluster %q state to InProgress", event.Id, cluster)
				}

				logger.Info().Msgf("Moved event %q for cluster %q into the work queue %q", event.Id, cluster, msg.Subject)

				metrics.TasksScheduled.Inc()

				// We break here as the config version was updated thus subsequent changes
				// will error out anyways. On the run next changes will be worked on.
				break clusters
			case spec.Workflow_IN_PROGRESS.String():
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

		pending, err := store.ConvertToGRPC(cfg)
		if err != nil {
			logger.Err(err).Msgf("Failed to convert from DB representation to Grpc: %q", name)
			continue
		}

		var desiredState map[string]*spec.Clusters
		if err := createDesiredState(pending, &desiredState); err != nil {
			logger.Err(err).Msgf("Failed to create desired state, skipping.")
			continue
		}

		ok, err := manifest.ValidStateTransitionString(pending.Manifest.State.String(), manifest.Scheduled)
		if err != nil || !ok {
			logger.Err(err).Msgf("Cannot transtition from manifest state %q to %q, skipping", pending.Manifest.State, manifest.Scheduled)
			continue
		}

		result := reconciliate(pending, desiredState)

		switch result {
		case Noop:
			logger.Debug().Msg("No task to be worked on, skip updating DB representation")
			continue
		case NotReady:
			logger.Info().Msgf("manifest is not ready to be scheduled, retrying again later")
		case Reschedule:
			pending.Manifest.State = spec.Manifest_Scheduled
			logger.Debug().Msgf("Scheduling for intermediate tasks after which the config will be rescheduled again")
		case NoReschedule:
			logger.Debug().Msgf("Scheduling for tasks after which the config will not be rescheduled again")
			pending.Manifest.State = spec.Manifest_Scheduled
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
		case Reschedule, NoReschedule:
			logger.Info().Msgf("Config has been successfully processed and moved to the %q state", manifest.Scheduled.String())
		}
	}

	return nil
}

func (s *Service) WatchForDoneOrErrorDocuments(ctx context.Context) error {
	cfgs, err := s.store.ListConfigs(ctx, &store.ListFilter{
		ManifestState: []string{
			manifest.Done.String(),
			manifest.Error.String(),
		},
	})
	if err != nil {
		return err
	}

	for _, idle := range cfgs {
		logger := loggerutils.WithProjectName(idle.Name)

		// While reconciliation should be done.
		if !bytes.Equal(idle.Manifest.LastAppliedChecksum, idle.Manifest.Checksum) {
			logger.
				Info().
				Msgf("Moving to %q as changes have been made to the manifest since the last build", manifest.Pending.String())

			ok, err := manifest.ValidStateTransitionString(idle.Manifest.State, manifest.Pending)
			if err != nil || !ok {
				logger.
					Err(err).
					Msgf("Cannot transtition from manifest state %q to %q, skipping", idle.Manifest.State, manifest.Pending)
				continue
			}

			idle.Manifest.State = manifest.Pending.String()

			for cluster, state := range idle.Clusters {
				currentEmpty := len(state.Current.K8s) == 0 && len(state.Current.LoadBalancers) == 0
				inFlightEmpty := state.InFlight == nil

				if currentEmpty && inFlightEmpty {
					logger.Debug().Msgf("Deleting cluster %q from database as infrastructure was destroyed", cluster)
					delete(idle.Clusters, cluster)
				}
			}

			if err := s.store.UpdateConfig(ctx, idle); err != nil {
				if errors.Is(err, store.ErrNotFoundOrDirty) {
					logger.Warn().Msgf("Idle Config couldn't be updated due to a Dirty Write, another retry will start shortly.")
					continue
				}
				logger.Err(err).Msgf("Failed to update idle config, skipping.")
				continue
			}

			logger.Info().Msgf("Config has been successfully processed and moved to the %q state", manifest.Pending.String())
			continue
		}

		// When reconciliation ends and the manifest should no longer be reconciliated.
		if idle.Manifest.State == manifest.Done.String() {
			if idle.Manifest.Checksum == nil && idle.Manifest.LastAppliedChecksum == nil {
				if err := s.store.DeleteConfig(ctx, idle.Name, idle.Version); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Warn().Msgf("Idle Config couldn't be deleted due to a Dirty Write, another retry will start shortly.")
						continue
					}
					logger.Err(err).Msgf("Failed to delete idle config, skipping.")
				}
				continue
			}
		}
	}

	return nil
}

func messageForStage(
	inputManifestName, clusterName, eventID string,
	marshalledTask []byte,
	stage *spec.Stage,
) (nats.Msg, string, error) {
	var (
		task         spec.Task
		work         spec.Work
		subject      string
		replySubject string
		description  string
	)

	if err := proto.Unmarshal(marshalledTask, &task); err != nil {
		return nats.Msg{}, "", err
	}

	work.Task = &task

	switch stage := stage.GetStageKind().(type) {
	case *spec.Stage_Ansibler:
		b := new(strings.Builder)
		b.WriteString(stage.Ansibler.Description.About)

		for _, pass := range stage.Ansibler.GetSubPasses() {
			r, err := anypb.New(pass)
			if err != nil {
				return nats.Msg{}, "", err
			}
			work.Passes = append(work.Passes, r)
			b.WriteByte('\n')
			fmt.Fprintf(b, "- %s", pass.Description.About)
		}

		description = b.String()
		subject = natsutils.AnsiblerRequests
		replySubject = natsutils.AnsiblerResponse
	case *spec.Stage_KubeEleven:
		b := new(strings.Builder)
		b.WriteString(stage.KubeEleven.Description.About)

		for _, pass := range stage.KubeEleven.GetSubPasses() {
			r, err := anypb.New(pass)
			if err != nil {
				return nats.Msg{}, "", err
			}
			work.Passes = append(work.Passes, r)
			b.WriteByte('\n')
			fmt.Fprintf(b, "- %s", pass.Description.About)
		}

		description = b.String()
		subject = natsutils.KubeElevenRequests
		replySubject = natsutils.KubeElevenResponse
	case *spec.Stage_Kuber:
		b := new(strings.Builder)
		b.WriteString(stage.Kuber.Description.About)

		for _, pass := range stage.Kuber.GetSubPasses() {
			r, err := anypb.New(pass)
			if err != nil {
				return nats.Msg{}, "", err
			}
			work.Passes = append(work.Passes, r)
			b.WriteByte('\n')
			fmt.Fprintf(b, "- %s", pass.Description.About)
		}

		description = b.String()
		subject = natsutils.KuberRequests
		replySubject = natsutils.KuberResponse
	case *spec.Stage_Terraformer:
		b := new(strings.Builder)
		b.WriteString(stage.Terraformer.Description.About)

		for _, pass := range stage.Terraformer.GetSubPasses() {
			r, err := anypb.New(pass)
			if err != nil {
				return nats.Msg{}, "", err
			}
			work.Passes = append(work.Passes, r)
			b.WriteByte('\n')
			fmt.Fprintf(b, "- %s", pass.Description.About)
		}

		description = b.String()
		subject = natsutils.TerraformerRequests
		replySubject = natsutils.TerraformerResponse
	default:
		return nats.Msg{}, "", fmt.Errorf("no mapping exists for stage %T", stage)
	}

	opts := proto.MarshalOptions{
		Deterministic: true,
	}

	data, err := opts.Marshal(&work)
	if err != nil {
		return nats.Msg{}, "", err
	}

	// Duplicate messages are tracked jetstream-wide
	// thus each stage needs its own ID for it to not
	// be considered as a duplicate if send to another
	// stage.
	stageID := fmt.Sprintf("%v-%v", eventID, subject)

	headers := nats.Header{}
	headers.Set(nats.MsgIdHdr, stageID)
	headers.Set(natsutils.ReplyToHeader, replySubject)
	headers.Set(natsutils.InputManifestName, inputManifestName)
	headers.Set(natsutils.ClusterName, clusterName)

	msg := nats.Msg{
		Subject: subject,
		Header:  headers,
		Data:    data,
	}

	return msg, description, nil
}
