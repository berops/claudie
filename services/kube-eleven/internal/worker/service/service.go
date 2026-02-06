package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/internal/natsutils"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var (
	// Port on which the health check service will be listening on.
	Port = envs.GetOrDefaultInt("KUBE_ELEVEN_PORT", 50054)

	// SpawnProcessLimit is the number of processes concurrently executing tofu.
	SpawnProcessLimit = envs.GetOrDefaultInt("KUBE_ELEVEN_CONCURRENT_CLUSTERS", 7)

	// Durable Name of this service.
	DurableName = envs.GetOrDefault("KUBE_ELEVEN_DURABLE_NAME", "kube-eleven")

	// Name used for health checking via the grpc health check.
	HealthCheckReadinessName = envs.GetOrDefault("KUBE_ELEVEN_HEALTHCHECK_READINESS_SERVICE_NAME", "kube-eleven-readiness")
	HealthCheckLivenessName  = envs.GetOrDefault("KUBE_ELEVEN_HEALTHCHECK_LIVENESS_SERVICE_NAME", "kube-eleven-liveness")

	// Ack wait time in minutes for processing incoming NATS messages.
	AckWait = time.Duration(envs.GetOrDefaultInt("KUBE_ELEVEN_ACK_WAIT_TIME", 8)) * time.Minute
)

type grpcServer struct {
	tcpListener  net.Listener
	server       *grpc.Server
	healthServer *health.Server
}

type natsConsumer struct {
	natsclient *natsutils.Client
	inFlight   sync.WaitGroup
	loopExited <-chan struct{}
}

type Service struct {
	spawnProcessLimit *semaphore.Weighted

	gserver  *grpcServer
	consumer *natsConsumer

	done chan struct{}
}

func New(ctx context.Context, opts ...grpc.ServerOption) (*Service, error) {
	listenerAddress := net.JoinHostPort("0.0.0.0", fmt.Sprintf("%v", Port))
	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(ctx, "tcp", listenerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener on address %s: %w", listenerAddress, err)
	}

	// The client is created with default options which include
	// an infinite number of reconnects, thus until the service
	// itself is not killed, it should always try to stay up
	// and process incoming messages.
	client, err := natsutils.NewClientWithJetStream(envs.NatsClusterURL, envs.NatsClusterSize)
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("failed to connect to nats cluster at %s with size %v: %w", envs.NatsClusterURL, envs.NatsClusterSize, err)
	}

	grpcserver := grpcutils.NewGRPCServer(opts...)
	healthserver := health.NewServer()

	healthserver.SetServingStatus(HealthCheckReadinessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	healthserver.SetServingStatus(HealthCheckLivenessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcserver, healthserver)

	spawnLimit := semaphore.NewWeighted(int64(SpawnProcessLimit))

	gserver := grpcServer{
		tcpListener:  listener,
		server:       grpcserver,
		healthServer: healthserver,
	}

	consumerLoopChan := make(chan struct{})
	natsconsumer := natsConsumer{
		natsclient: client,
		loopExited: consumerLoopChan,
	}

	s := Service{
		spawnProcessLimit: spawnLimit,
		gserver:           &gserver,
		consumer:          &natsconsumer,
		done:              make(chan struct{}),
	}

	go s.consumerLoop(consumerLoopChan)
	return &s, nil
}

func (s *Service) ServeHealthChecks() error {
	if err := s.gserver.server.Serve(s.gserver.tcpListener); err != nil {
		return fmt.Errorf("failed to serve health checks: %w", err)
	}
	log.Info().Msg("Finished listening for incoming health checks")
	return nil
}

func (s *Service) Stop() {
	log.Info().Msg("Gracefully shutting down serivce")

	// signal we are closing to all spawned go-routines.
	close(s.done)

	// wait for the consumer loop to exit.
	<-s.consumer.loopExited

	// we are no longer processing any messages, close all other connections.
	s.consumer.natsclient.Close()
	s.gserver.server.GracefulStop()
	s.gserver.healthServer.Shutdown()
	s.gserver.tcpListener.Close()
}

func (s *Service) PerformHealthCheckAndUpdateStatus() {
	if status := s.consumer.natsclient.Conn().Status(); status != nats.CONNECTED {
		err := fmt.Errorf("nats connection status is %s", status.String())
		s.gserver.healthServer.SetServingStatus(HealthCheckReadinessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.gserver.healthServer.SetServingStatus(HealthCheckLivenessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		log.Debug().Msgf("Failed to verify healthcheck: %v", err)
		return
	}

	s.gserver.healthServer.SetServingStatus(HealthCheckReadinessName, grpc_health_v1.HealthCheckResponse_SERVING)
	s.gserver.healthServer.SetServingStatus(HealthCheckLivenessName, grpc_health_v1.HealthCheckResponse_SERVING)
}

func (s *Service) consumerLoop(exit chan<- struct{}) {
	defer close(exit)

	for {
		select {
		case <-s.done:
			log.
				Info().
				Msg("Closing consumer loop, received done signal, waiting for any pending processes to finish")

			s.consumer.inFlight.Wait()
			return
		default:
			log.Info().Msg("Creating consumer for incoming messages")
			// fallthrough.
		}

		var (
			creatingConsumerDone = make(chan struct{})
			ctx, cancel          = context.WithCancel(context.Background())
		)

		go func() {
			// on both consumer done or the service being killed, cancel the context.
			defer cancel()

			for {
				select {
				case <-creatingConsumerDone:
					return
				case <-s.done:
					return
				}
			}
		}()

		consumer, err := s.consumer.natsclient.JSWorkQueueConsumer(
			ctx,
			DurableName,
			envs.NatsClusterJetstreamName,
			AckWait,
			natsutils.KubeElevenRequests,
		)

		close(creatingConsumerDone)
		<-ctx.Done()

		if err != nil {
			jitter := rand.IntN(500)
			sleep := 1850*time.Millisecond + (time.Duration(jitter) * time.Millisecond)

			log.
				Err(err).
				Msgf("Failed to create work queue consumer, will try again in %s", sleep)
			time.Sleep(sleep)
			continue
		}

		consumeOptions := [...]jetstream.PullConsumeOpt{
			jetstream.ConsumeErrHandler(errHandler),
			// The consumer will by default buffer messages behind the scenes and if the messages are not
			// acknowledged, even if they're buffered, within the specified ack timeout they will be re-send,
			// thus we always keep a maximum of 1 message to be buffered. To then handle multiple msgs at once
			// we process each message in each go-routine.
			jetstream.PullMaxMessages(1),
		}

		cctx, err := consumer.Consume(s.Handler, consumeOptions[:]...)
		if err != nil {
			jitter := rand.IntN(500)
			sleep := 850*time.Millisecond + (time.Duration(jitter) * time.Millisecond)

			log.
				Err(err).
				Msgf("Failed to start consuming messages, will try again in %s", sleep)
			time.Sleep(sleep)
			continue
		}

		log.Info().Msg("Consumer created and registered for incoming messages")

		// Everything was created Okay, now explicitly wait for something to stop
		// the consuming.
		select {
		case <-cctx.Closed():
			log.
				Info().
				Msg("Current consumer stopped. Waiting for any processing to finish, will recreate later")

			s.consumer.inFlight.Wait()

			// Since the service did not exit, some outside
			// interference must be going on or some unknown
			// errors, thus continue with the consumer loop.
			continue
		case <-s.done:
			// unsubscribe and discard any buffered messages in NATS.
			cctx.Stop()

			// wait for current in-filght messages to finish.
			s.consumer.inFlight.Wait()

			// wait for the consumer to close.
			<-cctx.Closed()

			log.Info().Msg("Closing consumer loop, received done signal")
			return
		}
	}
}

func errHandler(consumeCtx jetstream.ConsumeContext, err error) {
	if errors.Is(err, nats.ErrConsumerDeleted) {
		log.
			Warn().
			Msgf("Received consumer error: %s, closing down current consumer", err.Error())

		consumeCtx.Stop()
		return
	}

	if errors.Is(err, nats.ErrNoResponders) {
		// [nats.ErrNoResponders] is not a terminal error
		// thus simply log in debug builds.
		//
		// Source: https://github.com/nats-io/nats.go/discussions/1158
		log.
			Debug().
			Msgf("Received error no responders: %v", err)
		return
	}

	if errors.Is(err, nats.ErrNoHeartbeat) {
		log.Warn().Msgf("%s", err)
		return
	}

	log.
		Err(err).
		Msgf("Failed to consume message: %v", err)
}
