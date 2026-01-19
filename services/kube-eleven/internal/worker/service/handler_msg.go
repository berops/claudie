package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/natsutils"
	"github.com/berops/claudie/internal/processlimit"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/semaphore"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func (s *Service) Handler(msg jetstream.Msg) {
	handler := func() {
		handlerInner(AckWait, s.spawnProcessLimit, s.done, s.consumer.natsclient.JetStream(), msg)
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
) {
	var (
		stageID       = msg.Headers().Get(nats.MsgIdHdr)
		suffix        = fmt.Sprintf("-%v", natsutils.KubeElevenRequests)
		parsedStageID = strings.Split(stageID, suffix)
		discard       = false
	)

	if len(parsedStageID) < 1 || parsedStageID[0] == "" {
		discard = true
		parsedStageID = []string{"unknown"}
	}

	if _, err := uuid.Parse(parsedStageID[0]); err != nil {
		discard = true
	}

	var (
		eventID           = parsedStageID[0]
		replyChannel      = msg.Headers().Get(natsutils.ReplyToHeader)
		inputManifestName = msg.Headers().Get(natsutils.InputManifestName)
		clusterName       = msg.Headers().Get(natsutils.ClusterName)

		logger = log.With().
			Str(natsutils.ClusterName, clusterName).
			Str(natsutils.InputManifestName, inputManifestName).
			Str(nats.MsgIdHdr, eventID).
			Logger()
	)

	if discard {
		reply := natsutils.ReplyMsg{
			InputManifest: inputManifestName,
			Cluster:       clusterName,
			TaskID:        eventID,
			Subject:       replyChannel,
		}
		// Try to send a noop as we failed to unmarshal the received message
		// if that fails we will get the same message re-delivered.
		natsutils.TryReplyErrorFTL(logger, errors.New("nats message with unknown/unsupported/missing headers received"), reply, js, msg)
		return
	}

	if err := msg.InProgress(); err != nil {
		logger.Warn().Msg("Failed perform first Progress refresh")
	}

	var work spec.Work
	if err := proto.Unmarshal(msg.Data(), &work); err != nil {
		logger.Err(err).Msg("Failed to unmarshal received message")
		reply := natsutils.ReplyMsg{
			InputManifest: inputManifestName,
			Cluster:       clusterName,
			TaskID:        eventID,
			Subject:       replyChannel,
		}
		// Try to send a noop as we failed to unmarshal the received message
		// if that fails we will get the same message re-delivered.
		natsutils.TryReplyErrorFTL(logger, err, reply, js, msg)
		return
	}

	var kubeelevenWork Work
	{
		kubeelevenWork.InputManifestName = inputManifestName

		kubeelevenWork.Task = work.Task
		work.Task = nil

		for _, pass := range work.Passes {
			var stage spec.StageKubeEleven_SubPass
			if err := anypb.UnmarshalTo(pass, &stage, proto.UnmarshalOptions{}); err != nil {
				logger.Err(err).Msg("Failed to unmarshal received stage for work")
				reply := natsutils.ReplyMsg{
					InputManifest: inputManifestName,
					Cluster:       clusterName,
					TaskID:        eventID,
					Subject:       replyChannel,
				}
				// Try to send a noop as we failed to unmarshal the received message
				// if that fails we will get the same message re-delivered.
				natsutils.TryReplyErrorFTL(logger, err, reply, js, msg)
				return
			}
			kubeelevenWork.Passes = append(kubeelevenWork.Passes, &stage)
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
				logger.Debug().Msg("Issuing InProgress refresh of the task")
				if err := msg.InProgress(); err != nil {
					logger.Err(err).Msg("Failed to issue InProgress refresh of the task")
				}
			}
		}
	}()

	result := ProcessTask(ctx, kubeelevenWork)

	close(processingDone)
	<-ctx.Done()

	var (
		err error

		reply = natsutils.ReplyMsg{
			InputManifest: inputManifestName,
			Cluster:       clusterName,
			TaskID:        eventID,
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

		if err = natsutils.ReplyTo(ctx, logger, js, reply); err == nil {
			break
		}

		jitter := time.Duration(rand.IntN(750)) * time.Millisecond
		wait := 5*time.Second + jitter
		wait = min(wait, refreshTime) // keep the refresh interval in mind.
		time.Sleep(wait)
	}

	if err != nil {
		// If failed to submit the result to the requested reply channel, do not
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
