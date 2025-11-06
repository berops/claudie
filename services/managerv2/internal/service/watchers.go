package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/berops/claudie/internal/api/manifest"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/natsutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/managerv2/internal/store"
	"github.com/nats-io/nats.go"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Tick represents the interval at which each manifest state is checked.
const Tick = 3 * time.Second

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
				continue clusters
			}

			if state.State.Status == spec.WorkflowV2_ERROR.String() {
				// We count as done as no further work will be done with
				// this cluster even though the state is in error.
				clustersDone++
				anyError = true
				continue clusters
			}

			event := state.Task

			noWork := event == nil
			noWork = noWork || len(event.Pipeline) < 1
			noWork = noWork || (int(event.CurrentStage) > len(event.Pipeline))
			if noWork {
				logger.Debug().Msgf("Nothing to be worked on for cluster %q, considering as done", cluster)

				state.Task = nil
				state.State.Status = spec.WorkflowV2_DONE.String()
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
			case spec.WorkflowV2_WAIT_FOR_PICKUP.String():
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
					logger.Warn().Msgf("event %q: %q, for cluster %q was submitted more than once but the duplication was caught", event.Id, event.Description, cluster)
					// if it was a duplicate we didn't managed to update the state in the DB last try, thus continue.
				}

				state.State.Status = spec.WorkflowV2_IN_PROGRESS.String()
				state.State.Description = fmt.Sprintf("%s: %s", event.Description, msgDescription)

				logger.Debug().Msgf("Moving event %q cluster %q state to InProgress", event.Id, cluster)
				if err := s.store.UpdateConfig(ctx, scheduled); err != nil {
					if errors.Is(err, store.ErrNotFoundOrDirty) {
						logger.Debug().Msgf("Failed to move event %q cluster %q state to InProgress, dirty write", event.Id, cluster)
						break
					}
					logger.Err(err).Msgf("Failed to move event %q cluster %q state to InProgress", event.Id, cluster)
				}

				logger.Info().Msgf("Moved event %q for cluster %q into the work queue %q", event.Id, cluster, msg.Subject)

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

		// TODO: I dont think we need to transfer current state
		// to the desired state here, only when we're about
		// to schedule the task, otherwise we should be able
		// to do the diff via just the namings.
		// This is to be done in the reconciliation loop.

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

func messageForStage(
	inputManifestName, clusterName, natsMsgId string,
	marshalledTask []byte,
	stage *spec.Stage,
) (nats.Msg, string, error) {
	var (
		task         spec.TaskV2
		work         spec.Work
		subject      string
		replySubject string
		description  string
	)

	err := proto.Unmarshal(marshalledTask, &task)
	if err != nil {
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
			fmt.Fprintf(b, "\t- %s", pass.Description.About)
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
			fmt.Fprintf(b, "\t- %s", pass.Description.About)
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
			fmt.Fprintf(b, "\t- %s", pass.Description.About)
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
			fmt.Fprintf(b, "\t- %s", pass.Description.About)
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

	headers := nats.Header{}
	headers.Set(nats.MsgIdHdr, natsMsgId)
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
