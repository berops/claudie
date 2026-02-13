package natsutils

import (
	"context"
	"fmt"
	"time"

	"github.com/berops/claudie/proto/pb/spec"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"

	"google.golang.org/protobuf/proto"
)

type ReplyMsg struct {
	// Name of the InputManifest for which the reply is targeted at.
	InputManifest string

	// Name of the cluster within the [ReplyMsg.InputManifest] for
	// which the reply is targeted at.
	Cluster string

	// TaskID is the ID from the picked up [nats.Msg], that was received
	// via the [nats.MsgIdHdr]. This is the actuall ID of the task that was
	// scheduled by the manager and this information is given back to the reply
	// channel in the header [WorkID]. This ID is also used to construct the ID
	// of the reply message, as each deplication tracking of messages is stream-wide
	// each stage needs to have its own ID, thus a simply concatenation of the
	// fmt.Sprintf("%v-%v", TaskID, Subject) is used.
	TaskID string

	// To which subject should the reply be send to.
	Subject string

	// Result of the processed task.
	Result *spec.TaskResult
}

func ReplyTo(
	ctx context.Context,
	logger zerolog.Logger,
	js jetstream.JetStream,
	result ReplyMsg,
) error {
	if result.Subject == ReplyDiscard {
		logger.Warn().Msg("Message does not have a reply channel attached, result is discarded")
		return nil
	}

	b, err := proto.Marshal(result.Result)
	if err != nil {
		return err
	}

	// Duplicate messages are tracked jetstream-wide thus each stage
	// needs its own ID for it to not be considered as a duplicate if
	// send to another stage.
	replyMsgID := fmt.Sprintf("%v-%v", result.TaskID, result.Subject)

	headers := nats.Header{}
	headers.Set(nats.MsgIdHdr, replyMsgID)
	headers.Set(WorkID, result.TaskID)
	headers.Set(InputManifestName, result.InputManifest)
	headers.Set(ClusterName, result.Cluster)

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

// Utility function used to send back a message that signals an unrecoverable fatal error with
// no changes made to the infrastructure for the received task to. The passed in `msg` is acknowledged
// and the Noop Reply is send to the targeted `replyChannel` channel.
func TryReplyErrorFTL(
	logger zerolog.Logger,
	err error,
	reply ReplyMsg,
	js jetstream.JetStream,
	msg jetstream.Msg,
) {
	reply.Result = &spec.TaskResult{
		Error: &spec.TaskResult_Error{
			Kind:        spec.TaskResult_Error_FATAL,
			Description: err.Error(),
		},
		Result: &spec.TaskResult_None_{None: new(spec.TaskResult_None)},
	}

	logger = logger.With().
		Str("reply-msg-id", msg.Headers().Get(nats.MsgIdHdr)).
		Str(ReplyToHeader, reply.Subject).Logger()

	// Send a reply and wait for an ack within the next 10 seconds, which should be genereous enough.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := ReplyTo(ctx, logger, js, reply); err != nil {
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
