package service

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/natsutils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"google.golang.org/protobuf/proto"
)

func (s *Service) Handler(msg jetstream.Msg) {
	handler := func() {
		stores := Stores{
			store: s.store,
		}
		handlerInner(AckWait, s.done, msg, stores)
	}

	s.nts.inFlight.Go(handler)
}

func errHandler(consumeCtx jetstream.ConsumeContext, err error) {
	log.Err(err).Msgf("Failed to consume message: %v", err)
}

func handlerInner(
	ackWait time.Duration,
	done chan struct{},
	msg jetstream.Msg,
	stores Stores,
) {
	var (
		// Messages are in the format (ID)-(SUBJECT), this is to avoid duplicate
		// messageId catching as the task moves trough the pipeline, thus in here
		// strip the suffix.
		subject           = msg.Subject()
		msgID             = strings.TrimSuffix(msg.Headers().Get(nats.MsgIdHdr), fmt.Sprintf("-%s", subject))
		taskID            = msg.Headers().Get(natsutils.WorkID)
		inputManifestName = msg.Headers().Get(natsutils.InputManifestName)
		clusterName       = msg.Headers().Get(natsutils.ClusterName)

		logger = log.With().
			Str(natsutils.InputManifestName, inputManifestName).
			Str(natsutils.ClusterName, clusterName).
			Str(natsutils.WorkID, taskID).
			Str(nats.MsgIdHdr, msgID).
			Str("msg-from-subject", subject).
			Logger()
	)

	if inputManifestName == "" {
		logger.
			Error().
			Msg("Message pulled from jetstream has missing [InputManifestName], won't process")
		discard(logger, msg)
		return
	}

	if clusterName == "" {
		logger.
			Error().
			Msg("Message pulled from jetstream has missing [ClusterName], won't process")
		discard(logger, msg)
		return
	}

	if taskID == "" {
		logger.
			Error().
			Msg("Message pulled from jetstream has missing ID of the task for which it was scheduled, won't process")
		discard(logger, msg)
		return
	}

	stage, ok := taskStageFromNatsSubject(subject)
	if !ok {
		logger.
			Error().
			Msg("Message pulled from jetstream has missing subject, won't process")
		discard(logger, msg)
		return
	}

	if err := msg.InProgress(); err != nil {
		logger.Warn().Msg("Failed to perform first Progress refresh")
	}

	var result spec.TaskResult
	if err := proto.Unmarshal(msg.Data(), &result); err != nil {
		logger.Err(err).Msg("Failed to unmarshal received message")
		discard(logger, msg)
		return
	}

	var (
		// perform InProgress notification every 10th of an interval of the whole `ackWait`.
		refreshTime    = max(ackWait*10/100, 100*time.Millisecond)
		processingDone = make(chan struct{})
		ctx, cancel    = context.WithCancel(context.Background())
		work           = Work{
			InputManifest: inputManifestName,
			Cluster:       clusterName,
			TaskID:        taskID,
			Stage:         stage,
			Result:        &result,
		}
	)

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

	acknowledge := ProcessTask(ctx, stores, work)

	close(processingDone)
	<-ctx.Done()

	if !acknowledge {
		logger.
			Warn().
			Msg("Processing task ended resulted in not acknowledging it, sending a NAK and waiting for re-delivery")

		if err := msg.Nak(); err != nil {
			logger.Err(err).Msg("Failed to send NAK, will wait for [AckWait] to expire")
		}
		return
	}

	var (
		err      error
		retries  = 5
		deadline = time.Now().Add(1 * time.Minute)
	)

	ctx, cancel = context.WithDeadline(context.Background(), deadline)
	defer cancel()

	for range retries {
		if err = msg.DoubleAck(ctx); err == nil {
			break
		}

		jitter := 1*time.Millisecond + (time.Duration(rand.IntN(100)) * time.Millisecond)
		time.Sleep(jitter)
	}

	if err != nil {
		logger.
			Err(err).
			Msg("Failed to acknowledge message after processing it, will wait for [AckWait] to expire for re-delivery")
	}

	logger.Info().Msg("Task processed")
}

func discard(logger zerolog.Logger, msg jetstream.Msg) {
	logger.Warn().Msg("Discarding received message")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := msg.DoubleAck(ctx); err != nil {
		logger.Err(err).Msg("failed to discard message, will wait for [AckWait] to expire for the re-delivery")
	}
}

// Maps the [natsutils.DefaultSubjects] to the [store.StageKind],
// Since we listen on multiple subjects we need to know from which
// stage the message came from, thus we simply do an inverse operation
// that was done in watchers.go:[messageForStage] function.
func taskStageFromNatsSubject(subject string) (store.StageKind, bool) {
	switch subject {
	case natsutils.TerraformerResponse:
		return store.Terraformer, true
	case natsutils.AnsiblerResponse:
		return store.Ansibler, true
	case natsutils.KubeElevenResponse:
		return store.KubeEleven, true
	case natsutils.KuberResponse:
		return store.Kuber, true
	default:
		return store.Unknown, false
	}
}
