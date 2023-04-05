package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/berops/claudie/proto/pb"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

type server struct {
	// router is the http multiplexer.
	router *http.ServeMux

	// server is the underlying http server.
	server *http.Server

	// manifestDir represents the path to the
	// directory the service will watch.
	manifestDir string

	// conn is the underlying connection used
	// by context-box client.
	conn *grpc.ClientConn

	// cBox is a gRPC client connection to the
	// context-box service.
	cBox pb.ContextBoxServiceClient

	// deletingConfigs is a go-routine safe map that
	// stores configs that are being currently deleted
	// to avoid having multiple go-routines deleting the
	// same config from the database.
	deletingConfigs sync.Map

	// groups is used to handle a graceful shutdown of the server.
	// It will wait for all spawned go-routines to finish their work.
	group sync.WaitGroup
}

func newServer(manifestDir string, service string) (*server, error) {
	// since the server will be responding to incoming requests we can't
	// use a blocking gRPC dial to the service thus we default to a non-blocking
	// connection with a RPC retry policy of ~4 minutes instead.
	opts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffExponentialWithJitter(4*time.Second, 0.2)),
		grpc_retry.WithMax(7),
		grpc_retry.WithCodes(codes.Unavailable),
	}

	conn, err := grpc.Dial(
		service,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(opts...)),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(opts...)),
	)

	if err != nil {
		return nil, err
	}

	s := &server{
		router:      http.NewServeMux(),
		manifestDir: manifestDir,
		conn:        conn,
		cBox:        pb.NewContextBoxServiceClient(conn),
	}

	s.routes()

	s.server = &http.Server{Handler: s.router, ReadHeaderTimeout: 2 * time.Second}

	return s, s.healthcheck()
}

func (s *server) GracefulShutdown() error {
	// First shutdown the http server to block any incoming connections.
	if err := s.server.Shutdown(context.Background()); err != nil {
		return err
	}

	// Wait for all go-routines to finish their work.
	s.group.Wait()

	// Finally close the connection to the context-box.
	return s.conn.Close()
}

func (s *server) ListenAndServe(addr string, port int) error {
	s.server.Addr = net.JoinHostPort(addr, fmt.Sprint(port))
	return s.server.ListenAndServe()
}

func (s *server) routes() {
	s.router.HandleFunc("/reload", s.handleReload(log.Logger))
}

// healthCheck checks if the manifestDir exists and the underlying gRPC
// connection to the context-box service is valid. As long as the directory
// exists and the connection is healthy the service is considered healthy.
func (s *server) healthcheck() error {
	if _, err := os.Stat(s.manifestDir); os.IsNotExist(err) {
		return fmt.Errorf("%v: %w", s.manifestDir, err)
	}

	if s.conn.GetState() == connectivity.Shutdown {
		return errors.New("unhealthy connection to context-box")
	}

	return nil
}

// handleReload handles incoming notifications from k8s-sidecar about changes
// (CREATE, UPDATE, DELETE) in configs in the specified manifest directory.
func (s *server) handleReload(logger zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			logger.Error().Msg("Received request method that is not HTTP GET")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		logger.Debug().Msgf("Received notification about change in the directory %s", s.manifestDir)
		s.group.Add(1)

		go func() {
			defer s.group.Done()

			if err := s.processConfigs(); err != nil {
				logger.Error().Msgf("Failed processing configs from directory %s : %v", s.manifestDir, err)
			}
		}()

		w.WriteHeader(http.StatusOK)
	}
}
