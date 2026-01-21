package natsutils

import (
	"context"
	"errors"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
)

// Default options used in the [NewClientWithJetStream] func.
var defaultOpts = [...]nats.Option{
	nats.MaxReconnects(-1), // endless reconnect attempts.
	nats.ReconnectWait(3 * time.Second),
	nats.ReconnectJitter(350*time.Millisecond, 1*time.Second),
	nats.DisconnectErrHandler(func(c *nats.Conn, err error) {
		log.Warn().Msgf("Disconnected from the NATS.io cluster: error %v\n", err)
	}),
	nats.ReconnectHandler(func(c *nats.Conn) {
		log.Info().Msgf("Reconnected to the NATS.io cluster: %q\n", c.ConnectedUrl())
	}),
	nats.ClosedHandler(func(c *nats.Conn) {
		log.Info().Msgf("Closed connection to the NATS.io cluster: error: %v\n", c.LastError())
	}),
	nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, natsError error) {
		log.Error().Msgf("Error: %v\n", natsError)
		if errors.Is(natsError, nats.ErrSlowConsumer) {
			pendingMsgs, _, err := sub.Pending()
			if err != nil {
				log.Error().Msgf("Couldn't get pending messages: %v", err)
				return
			}
			log.Warn().Msgf("Falling behind with %d pending messages on subject %q.\n", pendingMsgs, sub.Subject)
		}
	}),
}

type Client struct {
	url         string
	clusterSize int
	conn        *nats.Conn
	js          jetstream.JetStream
}

func NewClientWithJetStream(url string, size int, opts ...nats.Option) (*Client, error) {
	if url == "" {
		url = envs.NatsClusterURL
	}
	if size <= 0 {
		size = envs.NatsClusterSize
	}
	options := defaultOpts[:]

	// this will also result in the overwrite of any of the default ones as they
	// will occur later in the slice and thus overwrite the previous value set.
	options = append(options, opts...)

	conn, err := nats.Connect(url, options...)
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(conn)
	if err != nil {
		return nil, err
	}

	out := &Client{
		url:         url,
		clusterSize: size,
		conn:        conn,
		js:          js,
	}

	return out, nil
}

func (c *Client) Conn() *nats.Conn               { return c.conn }
func (c *Client) Close()                         { c.conn.Close() }
func (c *Client) JetStream() jetstream.JetStream { return c.js }

// Creates a new [jetstream.JetStream] instance with sensible default values.
// If a jetstream with given name already exists and its configuration differs from
// the provided one, it will be updated.
func (c *Client) JetStreamWorkQueue(ctx context.Context, name string, subjects ...string) error {
	if len(subjects) == 0 {
		subjects = DefaultSubjects[:]
	}

	defaultConfig := jetstream.StreamConfig{
		Name:     name,
		Subjects: subjects,
		// Retention type WorkQueuePolicy so that the messages are retained until acknowledged.
		Retention:            jetstream.WorkQueuePolicy,
		MaxMsgs:              16384,                // have up to 8192 messages across all subjects.
		Discard:              jetstream.DiscardNew, // when full discard new messages while keeping the old.
		DiscardNewPerSubject: true,                 // Discard also new messages per subject when the queue is full already.
		MaxMsgsPerSubject:    4096,                 // Limit the number of messages to 2048 per each subject.

		// Persist the incoming messages on disk, rather than in-memory.
		// So that un-expected restarts do not invalidate our state.
		Storage: jetstream.FileStorage,

		Replicas:    c.clusterSize,
		Duplicates:  5 * time.Minute, // keep track of duplicate messages for 5 mins.
		Compression: jetstream.NoCompression,
		ConsumerLimits: jetstream.StreamConsumerLimits{
			InactiveThreshold: 30 * time.Second,
			MaxAckPending:     512,
		},
		PersistMode: jetstream.DefaultPersistMode, // required every message to be flushed.
	}

	_, err := c.js.CreateOrUpdateStream(ctx, defaultConfig)
	return err
}

// Creates a Work Queue jetstream consumer with sensible default values.
func (c *Client) JSWorkQueueConsumer(
	ctx context.Context,
	durableName,
	stream string,
	ackWait time.Duration,
	subjects ...string,
) (jetstream.Consumer, error) {
	defaultConfig := jetstream.ConsumerConfig{
		Durable:       durableName,
		DeliverPolicy: jetstream.DeliverAllPolicy,  // required with work queue.
		AckPolicy:     jetstream.AckExplicitPolicy, // explicit ack required for messages.
		AckWait:       ackWait,
		ReplayPolicy:  jetstream.ReplayInstantPolicy, // resent messages instantly.
		MaxWaiting:    512,                           // alllow to have a total of 512 waiting messages
		MaxAckPending: 512,                           // allow to have a total of 512 unack messages pending
	}

	if len(subjects) == 1 {
		defaultConfig.FilterSubject = subjects[0]
	} else {
		defaultConfig.FilterSubjects = subjects
	}

	return c.js.CreateOrUpdateConsumer(ctx, stream, defaultConfig)
}
