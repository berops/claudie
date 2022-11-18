package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

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
					return fmt.Errorf("error while building cluster %s : %w", cluster.ClusterInfo.Name, err)
				}
				return nil
			})
		}(cluster, desiredState.LoadBalancerClusters)
	}
	if err := errGroup.Wait(); err != nil {
		log.Error().Msgf("Error encounter in BuildCluster for project %s : %s", desiredState.Name, err.Error())
		return &pb.BuildClusterResponse{DesiredState: desiredState, ErrorMessage: fmt.Sprintf("Error encounter in BuildCluster for project %s : %s", desiredState.Name, err.Error())}, err
	}
	log.Info().Msgf("Clusters for project %s were successfully build", desiredState.Name)
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

	g, ctx := errgroup.WithContext(context.Background())

	//goroutine for interrupt
	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)

		// wait for either the received signal or
		// check if an error occurred in other
		// go-routines.
		var err error
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case sig := <-ch:
			log.Info().Msgf("Received signal %v", sig)
			err = errors.New("kube-eleven interrupt signal")
		}

		log.Info().Msg("Gracefully shutting down gRPC server")
		s.GracefulStop()

		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)

		return err
	})

	//server goroutine
	g.Go(func() error {
		// s.Serve() will create a service goroutine for each connection
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("KubeEleven failed to serve: %w", err)
		}
		log.Info().Msg("Finished listening for incoming connections")
		return nil
	})

	log.Info().Msgf("Stopping KubeEleven: %s", g.Wait())
}
