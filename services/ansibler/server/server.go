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

	"github.com/berops/claudie/internal/healthcheck"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	defaultAnsiblerPort = 50053
)

type server struct {
	pb.UnimplementedAnsiblerServiceServer
}

// InstallNodeRequirements installs requirements on all nodes
func (*server) InstallNodeRequirements(_ context.Context, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	log.Info().Msgf("Installing node requirements for cluster %s project %s", req.Desired.ClusterInfo.Name, req.ProjectName)
	info := &NodepoolInfo{
		Nodepools:  req.Desired.ClusterInfo.NodePools,
		PrivateKey: req.Desired.ClusterInfo.PrivateKey,
		ID:         fmt.Sprintf("%s-%s", req.Desired.ClusterInfo.Name, req.Desired.ClusterInfo.Hash),
		Network:    req.Desired.Network,
	}

	if err := installLonghornRequirements(info); err != nil {
		log.Error().Msgf("Error encountered while installing node requirements for cluster %s project %s : %s", req.Desired.ClusterInfo.Name, req.ProjectName, err)
		return nil, fmt.Errorf("error encountered while installing node requirements for cluster %s project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	log.Info().Msgf("Node requirements for cluster %s project %s was successfully installed", req.Desired.ClusterInfo.Name, req.ProjectName)
	return &pb.InstallResponse{Desired: req.Desired, DesiredLbs: req.DesiredLbs}, nil
}

// InstallVPN installs VPN between nodes in the k8s cluster and lb clusters
func (*server) InstallVPN(_ context.Context, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	log.Info().Msgf("Installing VPN for cluster %s project %s", req.Desired.ClusterInfo.Name, req.ProjectName)
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

	if err := installWireguardVPN(req.Desired.ClusterInfo.Name, info); err != nil {
		log.Error().Msgf("Error encountered while installing VPN for cluster %s project %s : %v", req.Desired.ClusterInfo.Name, req.ProjectName, err)
		return nil, fmt.Errorf("error encountered while installing VPN for cluster %s project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	log.Info().Msgf("VPN for cluster %s project %s was successfully installed", req.Desired.ClusterInfo.Name, req.ProjectName)
	return &pb.InstallResponse{Desired: req.Desired, DesiredLbs: req.DesiredLbs}, nil
}

// TeardownLoadBalancers correctly destroys loadbalancers by selecting the new ApiServer endpoint
func (*server) TeardownLoadBalancers(ctx context.Context, req *pb.TeardownLBRequest) (*pb.TeardownLBResponse, error) {
	if len(req.DeletedLbs) == 0 {
		return &pb.TeardownLBResponse{
			PreviousAPIEndpoint: "",
			Desired:             req.Desired,
			DesiredLbs:          req.DesiredLbs,
			DeletedLbs:          req.DeletedLbs,
		}, nil
	}
	log.Info().Msgf("Tearing down the loadbalancers for cluster %s project %s", req.Desired.ClusterInfo.Name, req.ProjectName)

	var attached bool
	for _, lb := range req.DesiredLbs {
		if hasAPIServerRole(lb.Roles) {
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
		log.Error().Msgf("Error encountered while setting up the LoadBalancers for cluster %s project %s: %v", err, req.Desired.ClusterInfo.Name, req.ProjectName)
		return nil, fmt.Errorf("error encountered while tearing down loadbalancers for cluster %s project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	resp := &pb.TeardownLBResponse{
		PreviousAPIEndpoint: endpoint,
		Desired:             req.Desired,
		DesiredLbs:          req.DesiredLbs,
		DeletedLbs:          req.DeletedLbs,
	}
	log.Info().Msgf("Loadbalancers for cluster %s project %s were successfully torn down", req.Desired.ClusterInfo.Name, req.ProjectName)
	return resp, nil
}

// SetUpLoadbalancers sets up the loadbalancers, DNS and verifies their configuration
func (*server) SetUpLoadbalancers(_ context.Context, req *pb.SetUpLBRequest) (*pb.SetUpLBResponse, error) {
	log.Info().Msgf("Setting up the loadbalancers for cluster %s project %s", req.Desired.ClusterInfo.Name, req.ProjectName)
	currentLBs := make(map[string]*pb.LBcluster)
	for _, lb := range req.CurrentLbs {
		currentLBs[lb.ClusterInfo.Name] = lb
	}

	info := &LBInfo{
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
		log.Error().Msgf("Error encountered while setting up the loadbalancers for cluster %s project %s : %s", req.Desired.ClusterInfo.Name, req.ProjectName, err)
		return nil, fmt.Errorf("error encountered while setting up the loadbalancers for cluster %s project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	log.Info().Msgf("Loadbalancers for cluster %s project %s were successfully set up", req.Desired.ClusterInfo.Name, req.ProjectName)
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
	healthService := healthcheck.NewServerHealthChecker(ansiblerPort, "ANSIBLER_PORT", nil)
	grpc_health_v1.RegisterHealthServer(s, healthService)

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
