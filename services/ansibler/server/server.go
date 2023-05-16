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

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

const (
	defaultAnsiblerPort = 50053
)

type server struct {
	pb.UnimplementedAnsiblerServiceServer
}

func (*server) UpdateAPIEndpoint(_ context.Context, req *pb.UpdateAPIEndpointRequest) (*pb.UpdateAPIEndpointResponse, error) {
	logger := utils.CreateLoggerWithProjectAndClusterName(req.ProjectName, req.Current.ClusterInfo.Name)

	if req.Current == nil {
		return &pb.UpdateAPIEndpointResponse{Current: req.Current, Desired: req.Desired}, nil
	}

	logger.Info().Msgf("Updating api endpoint")
	if err := updateAPIEndpoint(req.Current.ClusterInfo, req.Desired.ClusterInfo); err != nil {
		return nil, fmt.Errorf("failed to update api endpoint for cluster %s project %s", req.Current.ClusterInfo.Name, req.ProjectName)
	}

	logger.Info().Msgf("Updated api endpoint")
	return &pb.UpdateAPIEndpointResponse{Current: req.Current, Desired: req.Desired}, nil
}

// InstallNodeRequirements installs requirements on all nodes
func (*server) InstallNodeRequirements(_ context.Context, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	logger := log.With().
		Str("project", req.ProjectName).Str("cluster", req.Desired.ClusterInfo.Name).
		Logger()

	logger.Info().Msgf("Installing node requirements")
	info := &NodepoolInfo{
		Nodepools:  req.Desired.ClusterInfo.NodePools,
		PrivateKey: req.Desired.ClusterInfo.PrivateKey,
		ID:         fmt.Sprintf("%s-%s", req.Desired.ClusterInfo.Name, req.Desired.ClusterInfo.Hash),
		Network:    req.Desired.Network,
	}

	if err := installLonghornRequirements(info); err != nil {
		logger.Err(err).Msgf("Error encountered while installing node requirements")
		return nil, fmt.Errorf("error encountered while installing node requirements for cluster %s project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	logger.Info().Msgf("Node requirements was successfully installed")
	return &pb.InstallResponse{Desired: req.Desired, DesiredLbs: req.DesiredLbs}, nil
}

// InstallVPN installs VPN between nodes in the k8s cluster and lb clusters
func (*server) InstallVPN(_ context.Context, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	logger := log.With().
		Str("project", req.ProjectName).Str("cluster", req.Desired.ClusterInfo.Name).
		Logger()

	logger.Info().Msgf("Installing VPN")
	info := &VPNInfo{
		Network: req.Desired.Network,
		NodepoolInfo: []*NodepoolInfo{
			{
				Nodepools:  req.Desired.ClusterInfo.NodePools,
				PrivateKey: req.Desired.ClusterInfo.PrivateKey,
				ID:         fmt.Sprintf("%s-%s", req.Desired.ClusterInfo.Name, req.Desired.ClusterInfo.Hash),
				Network:    req.Desired.Network,
			},
		},
	}

	for _, lb := range req.DesiredLbs {
		info.NodepoolInfo = append(info.NodepoolInfo, &NodepoolInfo{
			Nodepools:  lb.ClusterInfo.NodePools,
			PrivateKey: lb.ClusterInfo.PrivateKey,
			ID:         fmt.Sprintf("%s-%s", lb.ClusterInfo.Name, lb.ClusterInfo.Hash),
			Network:    req.Desired.Network,
		})
	}

	if err := installWireguardVPN(fmt.Sprintf("%s-%s", req.Desired.ClusterInfo.Name, req.Desired.ClusterInfo.Hash), info); err != nil {
		logger.Err(err).Msgf("Error encountered while installing VPN")
		return nil, fmt.Errorf("error encountered while installing VPN for cluster %s project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	logger.Info().Msgf("VPN was successfully installed")
	return &pb.InstallResponse{Desired: req.Desired, DesiredLbs: req.DesiredLbs}, nil
}

// TeardownLoadBalancers correctly destroys loadbalancers by selecting the new ApiServer endpoint
func (*server) TeardownLoadBalancers(ctx context.Context, req *pb.TeardownLBRequest) (*pb.TeardownLBResponse, error) {
	logger := log.With().
		Str("project", req.ProjectName).Str("cluster", req.Desired.ClusterInfo.Name).
		Logger()

	if len(req.DeletedLbs) == 0 {
		return &pb.TeardownLBResponse{
			PreviousAPIEndpoint: "",
			Desired:             req.Desired,
			DesiredLbs:          req.DesiredLbs,
			DeletedLbs:          req.DeletedLbs,
		}, nil
	}
	logger.Info().Msgf("Tearing down the loadbalancers")

	var attached bool
	for _, lb := range req.DesiredLbs {
		if utils.HasAPIServerRole(lb.Roles) {
			attached = true
		}
	}

	// for each load-balancer that is being deleted collect LbData.
	info := &LBInfo{
		TargetK8sNodepool:    req.Desired.ClusterInfo.NodePools,
		TargetK8sNodepoolKey: req.Desired.ClusterInfo.PrivateKey,
		ClusterID:            fmt.Sprintf("%s-%s", req.Desired.ClusterInfo.Name, req.Desired.ClusterInfo.Hash),
	}
	for _, lb := range req.DeletedLbs {
		info.LbClusters = append(info.LbClusters, &LBData{
			DesiredLbCluster: nil,
			CurrentLbCluster: lb,
		})
	}

	endpoint, err := teardownLoadBalancers(req.Desired.ClusterInfo.Name, info, attached)
	if err != nil {
		logger.Err(err).Msgf("Error encountered while setting up the LoadBalancers")
		return nil, fmt.Errorf("error encountered while tearing down loadbalancers for cluster %s project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	resp := &pb.TeardownLBResponse{
		PreviousAPIEndpoint: endpoint,
		Desired:             req.Desired,
		DesiredLbs:          req.DesiredLbs,
		DeletedLbs:          req.DeletedLbs,
	}
	logger.Info().Msgf("Loadbalancers were successfully torn down")
	return resp, nil
}

// SetUpLoadbalancers sets up the loadbalancers, DNS and verifies their configuration
func (*server) SetUpLoadbalancers(_ context.Context, req *pb.SetUpLBRequest) (*pb.SetUpLBResponse, error) {
	logger := log.With().
		Str("project", req.ProjectName).Str("cluster", req.Desired.ClusterInfo.Name).
		Logger()

	logger.Info().Msgf("Setting up the loadbalancers for cluster")
	currentLBs := make(map[string]*pb.LBcluster)
	for _, lb := range req.CurrentLbs {
		currentLBs[lb.ClusterInfo.Name] = lb
	}

	info := &LBInfo{
		FirstRun:              req.FirstRun,
		TargetK8sNodepool:     req.Desired.ClusterInfo.NodePools,
		TargetK8sNodepoolKey:  req.Desired.ClusterInfo.PrivateKey,
		PreviousAPIEndpointLB: req.PreviousAPIEndpoint,
		ClusterID:             fmt.Sprintf("%s-%s", req.Desired.ClusterInfo.Name, req.Desired.ClusterInfo.Hash),
	}

	for _, lb := range req.DesiredLbs {
		info.LbClusters = append(info.LbClusters, &LBData{
			DesiredLbCluster: lb,
			// if there is a value in the map it will return it, otherwise nil is returned.
			CurrentLbCluster: currentLBs[lb.ClusterInfo.Name],
		})
	}

	if err := setUpLoadbalancers(req.Desired.ClusterInfo.Name, info); err != nil {
		logger.Err(err).Msgf("Error encountered while setting up the loadbalancers")
		return nil, fmt.Errorf("error encountered while setting up the loadbalancers for cluster %s project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	logger.Info().Msgf("Loadbalancers were successfully set up")
	return &pb.SetUpLBResponse{Desired: req.Desired, CurrentLbs: req.CurrentLbs, DesiredLbs: req.DesiredLbs}, nil
}

func main() {
	// initialize logger
	utils.InitLog("ansibler")
	// Set Ansibler port
	ansiblerPort := utils.GetenvOr("ANSIBLER_PORT", fmt.Sprint(defaultAnsiblerPort))
	serviceAddr := net.JoinHostPort("0.0.0.0", ansiblerPort)
	lis, err := net.Listen("tcp", serviceAddr)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %s : %v", serviceAddr, err)
	}
	log.Info().Msgf("Ansibler service is listening on %s", serviceAddr)
	// creating a new server
	s := grpc.NewServer()
	pb.RegisterAnsiblerServiceServer(s, &server{})

	// Add health service to gRPC
	healthServer := health.NewServer()
	// Ansibler does not have any custom health check functions, thus always serving.
	healthServer.SetServingStatus("ansibler-liveness", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("ansibler-readiness", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthServer)

	g, ctx := errgroup.WithContext(context.Background())

	// listen for system interrupts to gracefully shut down
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
			err = errors.New("interrupt signal")
		}

		log.Info().Msg("Gracefully shutting down gRPC server")
		s.GracefulStop()
		healthServer.Shutdown()

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
			return fmt.Errorf("ansibler failed to serve: %w", err)
		}
		log.Info().Msg("Finished listening for incoming connections")
		return nil
	})

	log.Info().Msgf("Stopping Ansibler: %v", g.Wait())
}
