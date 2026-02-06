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
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/manager/internal/store"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

var (
	// Port on which the grpc server will be listening on.
	Port = envs.GetOrDefaultInt("MANAGER_PORT", 50055)

	// Durable name of this service.
	DurableName = envs.GetOrDefault("MANAGER_DURABLE_NAME", "manager")

	// Name used for health checking via the grpc health check.
	HealthCheckReadinessName = envs.GetOrDefault("MANAGER_HEALTHCHECK_READINESS_SERVICE_NAME", "manager-readiness")
	HealthCheckLivenessName  = envs.GetOrDefault("MANAGER_HEALTHCHECK_LIVENESS_SERVICE_NAME", "manager-liveness")

	// Ack wait time in minutes for processing incoming NATS messages.
	AckWait = time.Duration(envs.GetOrDefaultInt("MANAGER_ACK_WAIT_TIME", 8)) * time.Minute
)

var _ pb.ManagerServiceServer = (*Service)(nil)

type grpcServer struct {
	tcpListener  net.Listener
	server       *grpc.Server
	healthServer *health.Server
}

type natsClient struct {
	client     *natsutils.Client
	inFlight   sync.WaitGroup
	loopExited <-chan struct{}
}

type Service struct {
	pb.UnimplementedManagerServiceServer

	store store.Store

	server *grpcServer
	nts    *natsClient

	done chan struct{}
}

func New(ctx context.Context, opts ...grpc.ServerOption) (*Service, error) {
	client, err := natsutils.NewClientWithJetStream(envs.NatsClusterURL, envs.NatsClusterSize)
	if err != nil {
		return nil, err
	}

	if err := client.JetStreamWorkQueue(ctx, envs.NatsClusterJetstreamName); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create/update %q queue", envs.NatsClusterJetstreamName)
	}

	log.Info().Msgf("jetstream %q successfully initialized", envs.NatsClusterJetstreamName)

	listeningAddress := net.JoinHostPort("0.0.0.0", fmt.Sprint(Port))

	lcfg := net.ListenConfig{}
	lis, err := lcfg.Listen(ctx, "tcp", listeningAddress)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to bind tcp socket for address: %q: %w", listeningAddress, err)
	}

	log.Info().Msgf("manager microservice bound to %s", listeningAddress)

	mongo, err := store.NewMongoClient(ctx, envs.DatabaseURL)
	if err != nil {
		client.Close()
		lis.Close()
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	if err := mongo.Init(); err != nil {
		client.Close()
		lis.Close()
		mongo.Close()
		return nil, fmt.Errorf("failed to init mongo database: %w", err)
	}

	grpcserver := grpcutils.NewGRPCServer(opts...)
	healthserver := health.NewServer()

	healthserver.SetServingStatus(HealthCheckReadinessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	healthserver.SetServingStatus(HealthCheckLivenessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcserver, healthserver)

	gserver := grpcServer{
		tcpListener:  lis,
		server:       grpcserver,
		healthServer: healthserver,
	}

	consumerLoopChan := make(chan struct{})
	natsconsumer := natsClient{
		client:     client,
		loopExited: consumerLoopChan,
	}

	s := &Service{
		store:  mongo,
		server: &gserver,
		nts:    &natsconsumer,
		done:   make(chan struct{}),
	}

	pb.RegisterManagerServiceServer(s.server.server, s)

	go s.consumerLoop(consumerLoopChan)
	go s.watchPending()
	go s.watchScheduled()
	go s.watchDoneOrError()

	return s, nil
}

// Serve will create a service goroutine for each connection
func (s *Service) Serve() error {
	if err := s.server.server.Serve(s.server.tcpListener); err != nil {
		return fmt.Errorf("manager microservice grpc server failed to serve: %w", err)
	}

	log.Info().Msgf("Finished listening for incoming gRPC connections")
	return nil
}

// Stop will gracefully shutdown the gRPC server and the healthcheck server
func (s *Service) Stop() error {
	log.Info().Msg("Gracefully shutting down service")

	// signal we are closing to all spawned go-routines.
	close(s.done)

	// wait for the consumer loop to exit.
	<-s.nts.loopExited

	s.nts.client.Close()
	s.server.server.GracefulStop()
	s.server.healthServer.Shutdown()

	err := s.server.tcpListener.Close()
	if errc := s.store.Close(); errc != nil {
		err = errors.Join(err, errc)
	}

	return err
}

func (s *Service) PerformHealthCheckAndUpdateStatus() {
	if err := s.store.HealthCheck(); err != nil {
		s.server.healthServer.SetServingStatus(HealthCheckReadinessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.server.healthServer.SetServingStatus(HealthCheckLivenessName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		log.Debug().Msgf("Failed to verify healthcheck: %v", err)
		return
	}
	s.server.healthServer.SetServingStatus(HealthCheckReadinessName, grpc_health_v1.HealthCheckResponse_SERVING)
	s.server.healthServer.SetServingStatus(HealthCheckLivenessName, grpc_health_v1.HealthCheckResponse_SERVING)
}

func (s *Service) watchPending() {
	for {
		select {
		case <-s.done:
			log.Info().Msg("Exited worker loop running WatchForPendingDocuments")
			return
		case <-time.After(PendingTick):
			if err := s.WatchForPendingDocuments(context.Background()); err != nil {
				log.Err(err).Msg("Watch for pending documents failed")
			}
		}
	}
}

func (s *Service) watchScheduled() {
	for {
		select {
		case <-s.done:
			log.Info().Msgf("Exited worker loop running WatchForScheduledDocuments")
			return
		case <-time.After(Tick):
			if err := s.WatchForScheduledDocuments(context.Background()); err != nil {
				log.Err(err).Msg("Watch for scheduled documents failed")
			}
		}
	}
}

func (s *Service) watchDoneOrError() {
	for {
		select {
		case <-s.done:
			log.Info().Msgf("Exited worker loop running WatchForDoneOrErrorDocuments")
			return
		case <-time.After(Tick):
			if err := s.WatchForDoneOrErrorDocuments(context.Background()); err != nil {
				log.Err(err).Msg("Watch for Done/Error documents failed")
			}
		}
	}
}

func (s *Service) consumerLoop(exit chan<- struct{}) {
	defer close(exit)

	for {
		select {
		case <-s.done:
			log.
				Info().
				Msg("Closing consumer loop, received done signal, waiting for any pending processes to finish")

			s.nts.inFlight.Wait()
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

		consumer, err := s.nts.client.JSWorkQueueConsumer(
			ctx,
			DurableName,
			envs.NatsClusterJetstreamName,
			AckWait,
			natsutils.AnsiblerResponse,
			natsutils.KuberResponse,
			natsutils.KubeElevenResponse,
			natsutils.TerraformerResponse,
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

			s.nts.inFlight.Wait()

			// Since the service did not exit, some outside
			// interference must be going on or some unknown
			// errors, thus continue with the consumer loop.
			continue
		case <-s.done:
			// unsubscribe and discard any buffered messages in NATS.
			cctx.Stop()

			// wait for current in-filght messages to finish.
			s.nts.inFlight.Wait()

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
