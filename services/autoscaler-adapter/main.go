package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/services/autoscaler-adapter/claudie_provider"
	"github.com/rs/zerolog/log"

	"google.golang.org/grpc"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"
)

var (
	projectName = os.Getenv("PROJECT_NAME")
	clusterName = os.Getenv("CLUSTER_NAME")
	port        = os.Getenv("ADAPTER_PORT")
)

func main() {
	if projectName == "" || clusterName == "" || port == "" {
		log.Fatal().Msgf("Env vars PROJECT_NAME and CLUSTER_NAME and ADAPTER_PORT must be specified")
	}

	loggerutils.Init(fmt.Sprintf("%s-%s", "autoscaler-adapter", clusterName))

	if err := run(); err != nil {
		log.Fatal().Msgf("Failed to run cluster-autoscaler adapter: %v", err)
	}
}

func run() error {
	server := grpcutils.NewGRPCServer(
		grpc.ChainUnaryInterceptor(grpcutils.PeerInfoInterceptor(&log.Logger)),
	)

	serviceAddr := net.JoinHostPort("0.0.0.0", port)

	//nolint
	lis, err := net.Listen("tcp", serviceAddr)
	if err != nil {
		return err
	}

	srv, err := claudie_provider.NewProvider(
		context.Background(),
		projectName,
		clusterName,
	)
	if err != nil {
		return err
	}

	protos.RegisterCloudProviderServer(server, srv)

	log.Info().Msgf("Server ready at: %s", port)
	return server.Serve(lis)
}
