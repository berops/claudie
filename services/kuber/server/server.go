package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/kuber/server/longhorn"
	"github.com/Berops/platform/services/kuber/server/secret"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const defaultKuberPort = 50057

type server struct {
	pb.UnimplementedKuberServiceServer
}

func (s *server) SetUpStorage(ctx context.Context, req *pb.SetUpStorageRequest) (*pb.SetUpStorageResponse, error) {
	desiredState := req.GetDesiredState()
	var errGroup errgroup.Group
	for _, cluster := range desiredState.GetClusters() {
		func(c *pb.K8Scluster) {
			errGroup.Go(func() error {
				longhorn := longhorn.Longhorn{Cluster: c}
				err := longhorn.SetUp()
				if err != nil {
					log.Error().Msgf("Error while setting up the longhorn for %s : %v", c.ClusterInfo.Name, err)
					return fmt.Errorf("error while setting up the longhorn")
				}
				return nil
			})
		}(cluster)
	}
	err := errGroup.Wait()
	if err != nil {
		return &pb.SetUpStorageResponse{DesiredState: desiredState, ErrorMessage: fmt.Sprintf("Error encountered in SetUpStorage: %v", err)}, err
	}
	return &pb.SetUpStorageResponse{DesiredState: desiredState, ErrorMessage: ""}, nil
}

func (s *server) StoreKubeconfig(ctx context.Context, req *pb.StoreKubeconfigRequest) (*pb.StoreKubeconfigResponse, error) {
	cluster := req.GetCluster()
	var errGroup errgroup.Group
	func(c *pb.K8Scluster) {
		errGroup.Go(func() error {
			sec := secret.New()
			// save kubeconfig as base64 encoded string
			sec.YamlManifest.Data.SecretData = base64.StdEncoding.EncodeToString([]byte(c.GetKubeconfig()))
			// create or update existing secret
			err := sec.Create()
			if err != nil {
				log.Error().Msgf("Error while creating the kubeconfig secret for %s", c.ClusterInfo.Name)
				return fmt.Errorf("error while creating kubeconfig secret")
			}
			return nil
		})
	}(cluster)
	err := errGroup.Wait()
	if err != nil {
		return &pb.StoreKubeconfigResponse{ErrorMessage: err.Error()}, err
	}
	return &pb.StoreKubeconfigResponse{ErrorMessage: ""}, nil
}

func main() {
	// initialize logger
	utils.InitLog("kuber", "GOLANG_LOG")

	// Set the kuber port
	kuberPort := utils.GetenvOr("KUBER_PORT", fmt.Sprint(defaultKuberPort))

	// Start Terraformer Service
	trfAddr := net.JoinHostPort("0.0.0.0", kuberPort)
	lis, err := net.Listen("tcp", trfAddr)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %v", err)
	}
	log.Info().Msgf("Kuber service is listening on: %s", trfAddr)

	s := grpc.NewServer()
	pb.RegisterKuberServiceServer(s, &server{})

	// Add health service to gRPC
	healthService := healthcheck.NewServerHealthChecker(kuberPort, "KUBER_PORT")
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, _ := errgroup.WithContext(context.Background())

	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)
		<-ch

		signal.Stop(ch)
		s.GracefulStop()

		return errors.New("kuber interrupt signal")
	})

	g.Go(func() error {
		// s.Serve() will create a service goroutine for each connection
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("kuber failed to serve: %v", err)
		}
		return nil
	})

	log.Info().Msgf("Stopping Kuber: %v", g.Wait())
}
