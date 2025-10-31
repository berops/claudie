package service

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/natsutils"
	"github.com/berops/claudie/internal/processlimit"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func (s *Service) Handler(msg jetstream.Msg) {
	handler := func() {
		stores := Stores{
			s3:     s.stateStorage,
			dynamo: s.dynamoDB,
		}
		handlerInner(AckWait, s.spawnProcessLimit, s.done, s.consumer.natsclient.JetStream(), msg, stores)
	}
	s.consumer.inFlight.Go(handler)
}

func errHandler(consumeCtx jetstream.ConsumeContext, err error) {
	log.Err(err).Msgf("Failed to consume message: %v", err)
}

func handlerInner(
	ackWait time.Duration,
	processLimit *semaphore.Weighted,
	done chan struct{},
	js jetstream.JetStream,
	msg jetstream.Msg,
	stores Stores,
) {
	var (
		replyMsgID        = uuid.New().String()
		taskID            = msg.Headers().Get(nats.MsgIdHdr)
		replyChannel      = msg.Headers().Get(natsutils.ReplyToHeader)
		inputManifestName = msg.Headers().Get(natsutils.InputManifestName)
		clusterName       = msg.Headers().Get(natsutils.ClusterName)

		logger = log.With().
			Str(natsutils.ClusterName, clusterName).
			Str(natsutils.InputManifestName, inputManifestName).
			Str(nats.MsgIdHdr, taskID).
			Logger()
	)

	if err := msg.InProgress(); err != nil {
		logger.Warn().Msg("Failed perform first Progress refresh")
	}

	var work spec.Work
	if err := proto.Unmarshal(msg.Data(), &work); err != nil {
		logger.Err(err).Msg("Failed to unmarshal received message")
		reply := ReplyMsg{
			InputManifest: inputManifestName,
			Cluster:       clusterName,
			TaskID:        taskID,
			ID:            replyMsgID,
			Subject:       replyChannel,
		}
		// Try to send a noop as we failed to unmarshal the received message
		// if that fails we will get the same message re-delivered.
		tryReplyNoop(logger, reply, js, msg)
		return
	}

	var terraformWork Work
	{
		terraformWork.InputManifestName = inputManifestName

		terraformWork.Task = work.Task
		work.Task = nil

		for _, pass := range work.Passes {
			var stage spec.StageTerraformer_SubPass
			if err := anypb.UnmarshalTo(pass, &stage, proto.UnmarshalOptions{}); err != nil {
				logger.Err(err).Msg("Failed to unmarshal received stage for work")
				reply := ReplyMsg{
					InputManifest: inputManifestName,
					Cluster:       clusterName,
					TaskID:        taskID,
					ID:            replyMsgID,
					Subject:       replyChannel,
				}
				// Try to send a noop as we failed to unmarshal the received message
				// if that fails we will get the same message re-delivered.
				tryReplyNoop(logger, reply, js, msg)
				return
			}
			terraformWork.Passes = append(terraformWork.Passes, &stage)
		}

		work.Passes = nil
	}

	var (
		processingDone = make(chan struct{})
		ctx, cancel    = context.WithCancel(context.Background())

		// perform InProgress notification every 10th of an interval of the whole `ackWait`.
		refreshTime = max(ackWait*10/100, 100*time.Millisecond)
	)

	ctx = processlimit.With(ctx, processLimit)
	ctx = loggerutils.With(ctx, logger)

	go func() {
		// on both task finished and service being killed, cancel the context.
		defer cancel()

		for {
			select {
			case <-processingDone:
				logger.Debug().Msg("Task finished processing")
				return
			case <-done:
				logger.Debug().Msg("Service being shutdown")
				return
			case <-time.After(refreshTime):
				logger.Debug().Msg("Issueing InProgress refresh of the task")
				if err := msg.InProgress(); err != nil {
					logger.Err(err).Msg("Failed to issue InProgress refresh of the task")
				}
			}
		}
	}()

	result := ProcessTask(ctx, stores, terraformWork)

	close(processingDone)
	<-ctx.Done()

	var (
		err error

		reply = ReplyMsg{
			InputManifest: inputManifestName,
			Cluster:       clusterName,
			TaskID:        taskID,
			ID:            replyMsgID,
			Subject:       replyChannel,
			Result:        result,
		}

		retries  = 5
		deadline = time.Now().Add(5 * time.Minute)
	)

	ctx, cancel = context.WithDeadline(context.Background(), deadline)
	defer cancel()

	for range retries {
		if err := msg.InProgress(); err != nil {
			log.Warn().Msgf("failed to refresh msg while trying to send result to its reply channel: %v", err)
		}

		if err = replyTo(ctx, logger, js, reply); err == nil {
			break
		}

		jitter := time.Duration(rand.IntN(750)) * time.Millisecond
		wait := 5*time.Second + jitter
		wait = min(wait, refreshTime) // keep the refresh interval in mind.
		time.Sleep(wait)
	}

	if err != nil {
		// If we failed to submit the result to the requested reply channel, do not
		// acknowledge the message and consider it as failed.
		logger.Err(err).Msgf("Failed to send task result to the requested reply channel: %q", reply.Subject)
		if err := msg.Nak(); err != nil {
			logger.Err(err).Msg("Failed to send Nak, will wait for AckWait to expire for the re-delivery")
		}
		return
	}

	if err := msg.InProgress(); err != nil {
		log.Err(err).Msg("Failed to refresh msg as in progress, after sending result to reply channel")
	}

	for range retries {
		if err = msg.DoubleAck(ctx); err == nil {
			break
		}

		jitter := 1*time.Millisecond + (time.Duration(rand.IntN(100)) * time.Millisecond)
		time.Sleep(jitter)
	}

	if err != nil {
		logger.Err(err).Msg("Failed to acknowledge message")
		// If we failed to acknowledge the message it will be re-delivered. As any
		// side-effects within this service are idempotent this won't introduce issues.
		return
	}

	logger.Info().Msg("Task processed")
}

type ReplyMsg struct {
	// Name of the InputManifest for which the reply is targeted at.
	InputManifest string

	// Name of the cluster within the [ReplyMsg.InputManifest] for
	// which the reply is targeted at.
	Cluster string

	// TaskID is the ID from the picked up [nats.Msg], that was received
	// via the [nats.MsgIdHdr]. This is the actuall ID of the task that was
	// scheduled and this information is given back to the reply channel in
	// the header [natsutils.WorkID].
	TaskID string

	// ID of the message that will be set as [nats.MsgIdHdr]
	// This must be unique even on re-delivery of the same message
	ID string

	// To which subject should the reply be send to.
	Subject string

	// Result of the processed task.
	Result *spec.TaskResult
}

func replyTo(
	ctx context.Context,
	logger zerolog.Logger,
	js jetstream.JetStream,
	result ReplyMsg,
) error {
	if result.Subject == natsutils.ReplyDiscard {
		logger.Warn().Msg("Message does not have a reply channel attached, result is discarded")
		return nil
	}

	b, err := proto.Marshal(result.Result)
	if err != nil {
		return err
	}

	headers := nats.Header{}
	headers.Set(nats.MsgIdHdr, result.ID)
	headers.Set(natsutils.WorkID, result.TaskID)
	headers.Set(natsutils.InputManifestName, result.InputManifest)
	headers.Set(natsutils.ClusterName, result.Cluster)

	msg := nats.Msg{
		Subject: result.Subject,
		Header:  headers,
		Data:    b,
	}

	ack, err := js.PublishMsg(ctx, &msg)
	if err != nil {
		return err
	}

	if ack.Duplicate {
		logger.Warn().Msg("Message was catched as a duplicate")
	}

	return nil
}

// acknowledges the passed in `msg` and replies a Noop to the targeted `replyChannel` channel.
func tryReplyNoop(logger zerolog.Logger, reply ReplyMsg, js jetstream.JetStream, msg jetstream.Msg) {
	reply.Result = &spec.TaskResult{
		Result: &spec.TaskResult_None_{None: new(spec.TaskResult_None)},
	}

	logger = logger.With().
		Str("reply-msg-id", reply.ID).
		Str(natsutils.ReplyToHeader, reply.Subject).Logger()

	// Send a reply and wait for an ack within the next 10 seconds, which should be genereous enough.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := replyTo(ctx, logger, js, reply); err != nil {
		logger.Err(err).Msg("Failed to send reply message")

		if err := msg.NakWithDelay(2 * time.Second); err != nil {
			logger.
				Err(err).
				Msg("Failed to Nak message, exiting, will wait for [AckWait] to expire for the re-delivery")
		}

		// Failed to publish to the requested reply to channel, return here and wait for the re-delivery
		return
	}

	logger.Debug().Msg("Successfully send noop reply")

	if err := msg.DoubleAck(ctx); err != nil {
		logger.Err(err).Msg("Failed to acknowledge message after sending NOOP reply")
		// if this fails just fallthrough here, we will get a re-delivery of the message after the [AckWait].
	}

	logger.Debug().Msg("Successfully acknowledged msg")
}
