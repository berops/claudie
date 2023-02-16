package main

import (
	"fmt"
	"net"
	"os"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/services/autoscaler-adapter/claudie_provider"
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
	utils.InitLog(fmt.Sprintf("%s-%s", clusterName, "claudie-adapter"))

	server := grpc.NewServer()

	// Listen
	serviceAddr := net.JoinHostPort("0.0.0.0", port)
	lis, err := net.Listen("tcp", serviceAddr)
	if err != nil {
		log.Fatal().Msgf("failed to listen: %s", err)
	}

	// Serve
	srv := claudie_provider.NewClaudieCloudProvider(projectName, clusterName)
	protos.RegisterCloudProviderServer(server, srv)
	log.Info().Msgf("Server ready at: %s", port)
	if err := server.Serve(lis); err != nil {
		log.Fatal().Msgf("failed to serve: %v", err)
	}

}
