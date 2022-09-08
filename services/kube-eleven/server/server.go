package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/Berops/claudie/internal/healthcheck"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	kubeEleven "github.com/Berops/claudie/services/kube-eleven/server/kube-eleven"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct {
	pb.UnimplementedKubeElevenServiceServer
}

const (
	defaultKubeElevenPort = 50054 // default port for kube-eleven
)

// BuildCluster builds all cluster defined in the desired state
func (*server) BuildCluster(_ context.Context, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	desiredState := req.GetDesiredState()
	log.Info().Msgf("BuildCluster request with Project name: %s", desiredState.GetName())
	var errGroup errgroup.Group

	// Build all clusters concurrently
	for _, cluster := range desiredState.GetClusters() {
		func(cluster *pb.K8Scluster, lbClusters []*pb.LBcluster) {
			errGroup.Go(func() error {
				ke := kubeEleven.KubeEleven{K8sCluster: cluster, LBClusters: lbClusters}
				err := ke.BuildCluster()
				if err != nil {
					log.Error().Msgf("error encountered in KubeEleven - BuildCluster: %v", err)
					return err
				}
				return nil
			})
		}(cluster, desiredState.LoadBalancerClusters)
	}
	err := errGroup.Wait()
	if err != nil {
		return &pb.BuildClusterResponse{DesiredState: desiredState, ErrorMessage: err.Error()}, err
	}
	return &pb.BuildClusterResponse{DesiredState: desiredState, ErrorMessage: ""}, nil
}

func main() {
	// initialize logger
	utils.InitLog("kube-eleven")
	// Set KubeEleven port
	kubeElevenPort := utils.GetenvOr("KUBE_ELEVEN_PORT", fmt.Sprint(defaultKubeElevenPort))
	kubeElevenAddr := net.JoinHostPort("0.0.0.0", kubeElevenPort)
	lis, err := net.Listen("tcp", kubeElevenAddr)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %s : %v", kubeElevenAddr, err)
	}
	log.Info().Msgf("KubeEleven service is listening on %s", kubeElevenAddr)

	s := grpc.NewServer()
	pb.RegisterKubeElevenServiceServer(s, &server{})

	// Add health service to gRPC
	healthService := healthcheck.NewServerHealthChecker(kubeElevenPort, "KUBE_ELEVEN_PORT", nil)
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, _ := errgroup.WithContext(context.Background())

	//goroutine for interrupt
	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		<-ch
		signal.Stop(ch)
		s.GracefulStop()

		return errors.New("KubeEleven interrupt signal")
	})

	//server goroutine
	g.Go(func() error {
		// s.Serve() will create a service goroutine for each connection
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("KubeEleven failed to serve: %v", err)
		}
		return nil
	})
	log.Info().Msgf("Stopping KubeEleven: %s", g.Wait())
}
