package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/services/autoscaler-adapter/claudie_provider"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
)

func main() {
	projectName := os.Getenv("PROJECT_NAME")
	clusterName := os.Getenv("CLUSTER_NAME")
	port := os.Getenv("ADAPTER_PORT")

	if projectName == "" || clusterName == "" || port == "" {
		log.Fatal().Msgf("Env vars PROJECT_NAME and CLUSTER_NAME and ADAPTER_PORT must be specified")
	}

	loggerutils.Init(fmt.Sprintf("%s-%s", "autoscaler-adapter", clusterName))

	server := grpcutils.NewGRPCServer(
		grpc.ChainUnaryInterceptor(grpcutils.PeerInfoInterceptor(&log.Logger)),
	)

	go func() {
		pprofMux := mux.NewRouter()

		pprofMux.HandleFunc("/debug/pprof/", pprof.Index).Methods(http.MethodGet)
		pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline).Methods(http.MethodGet)
		pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile).Methods(http.MethodGet)
		pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol).Methods(http.MethodGet)
		pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace).Methods(http.MethodGet)
		pprofMux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine")).Methods(http.MethodGet)
		pprofMux.Handle("/debug/pprof/heap", pprof.Handler("heap")).Methods(http.MethodGet)
		pprofMux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate")).Methods(http.MethodGet)
		pprofMux.Handle("/debug/pprof/block", pprof.Handler("block")).Methods(http.MethodGet)
		pprofMux.Handle("/debug/pprof/allocs", pprof.Handler("allocs")).Methods(http.MethodGet)
		pprofMux.Handle("/debug/pprof/mutex", pprof.Handler("mutex")).Methods(http.MethodGet)

		_ = http.ListenAndServe("0.0.0.0:"+"18000", pprofMux)
	}()

	// Listen
	serviceAddr := net.JoinHostPort("0.0.0.0", port)
	lis, err := net.Listen("tcp", serviceAddr)
	if err != nil {
		log.Fatal().Msgf("failed to listen: %s", err)
	}

	// Serve
	srv := claudie_provider.NewClaudieCloudProvider(context.Background(), projectName, clusterName)
	protos.RegisterCloudProviderServer(server, srv)
	log.Info().Msgf("Server ready at: %s", port)
	if err := server.Serve(lis); err != nil {
		log.Fatal().Msgf("failed to serve: %v", err)
	}
}
