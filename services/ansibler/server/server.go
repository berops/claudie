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

// InstallNodeRequirements installs any requirements there are on all of the nodes
func (*server) InstallNodeRequirements(_ context.Context, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	var k8sNodepools []*NodepoolInfo
	//add k8s nodes to k8sNodepools
	for _, cluster := range req.DesiredState.Clusters {
		k8sNodepools = append(k8sNodepools, &NodepoolInfo{Nodepools: cluster.ClusterInfo.NodePools, PrivateKey: cluster.ClusterInfo.PrivateKey, ID: cluster.ClusterInfo.Name})
	}
	//since all nodes need to have longhorn req installed, we do not need to sort them in any way
	if err := installLonghornRequirements(k8sNodepools); err != nil {
		log.Error().Msgf("Error encountered while installing node requirements for project %s : %s", req.DesiredState.Name, err.Error())
		return nil, fmt.Errorf("error encountered while installing node requirements for project %s : %s", req.DesiredState.Name, err.Error())
	}
	log.Info().Msgf("Node requirements for project %s were successfully installed", req.DesiredState.Name)
	return &pb.InstallResponse{DesiredState: req.DesiredState}, nil
}

// InstallVPN installs VPN between nodes in the k8s cluster and lb clusters
func (*server) InstallVPN(_ context.Context, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	vpnNodepools := make(map[string]*VPNInfo) //[k8sClusterName][]nodepoolInfos
	//add k8s nodepools to vpn nodepools
	for _, cluster := range req.DesiredState.Clusters {
		var np []*NodepoolInfo
		np = append(np, &NodepoolInfo{Nodepools: cluster.ClusterInfo.NodePools, PrivateKey: cluster.ClusterInfo.PrivateKey, ID: cluster.ClusterInfo.Name})
		vpnNodepools[cluster.ClusterInfo.Name] = &VPNInfo{Network: cluster.Network, NodepoolInfo: np}
	}
	//add LB nodepools to vpn nodepools, so LBs will be part of the VPN
	for _, lbCluster := range req.DesiredState.LoadBalancerClusters {
		if nodepoolInfos, ok := vpnNodepools[lbCluster.TargetedK8S]; ok {
			nodepoolInfos.NodepoolInfo = append(nodepoolInfos.NodepoolInfo, &NodepoolInfo{Nodepools: lbCluster.ClusterInfo.NodePools, PrivateKey: lbCluster.ClusterInfo.PrivateKey, ID: lbCluster.ClusterInfo.Name})
		}
	}
	//there will be N VPNs for N clusters, thus we sorted the nodes based on the k8s cluster name
	if err := installWireguardVPN(vpnNodepools); err != nil {
		log.Error().Msgf("Error encountered while installing VPN for project %s : %s", req.DesiredState.Name, err.Error())
		return nil, fmt.Errorf("error encountered while installing VPN for project %s : %s", req.DesiredState.Name, err.Error())
	}
	log.Info().Msgf("VPNs for project %s were successfully installed", req.DesiredState.Name)
	return &pb.InstallResponse{DesiredState: req.DesiredState}, nil
}

// SetUpLoadbalancers sets up the loadbalancers, DNS and verifies their configuration
func (*server) SetUpLoadbalancers(_ context.Context, req *pb.SetUpLBRequest) (*pb.SetUpLBResponse, error) {
	lbInfos := make(map[string]*LBInfo)             //[k8sClusterName]lbInfo
	k8sNodepools := make(map[string][]*pb.NodePool) //[k8sClusterName][]nodepools
	k8sNodepoolsKey := make(map[string]string)      //[k8sClusterName]keys
	currentDNS := make(map[string]*pb.DNS)          //[lbClusterName]dns - of the current state LB
	//get all nodepools from clusters
	for _, k8s := range req.DesiredState.Clusters {
		k8sNodepools[k8s.ClusterInfo.Name] = k8s.ClusterInfo.NodePools
		k8sNodepoolsKey[k8s.ClusterInfo.Name] = k8s.ClusterInfo.PrivateKey
	}
	//get current dns so we can detect a possible change in configuration
	for _, lb := range req.CurrentState.LoadBalancerClusters {
		currentDNS[lb.ClusterInfo.Name] = lb.Dns
	}
	//get lb data
	for _, lb := range req.DesiredState.LoadBalancerClusters {
		if np, ok := k8sNodepools[lb.TargetedK8S]; ok {
			var newLbInfo *LBInfo
			//check if any LB for this k8s have been found
			if oldLbInfo, ok := lbInfos[lb.TargetedK8S]; ok {
				newLbInfo = oldLbInfo
			} else {
				newLbInfo = &LBInfo{TargetK8sNodepool: np, LbClusters: make([]*LBData, 0), TargetK8sNodepoolKey: k8sNodepoolsKey[lb.TargetedK8S]}
			}
			lbData := &LBData{LbCluster: lb}
			//check if dns in current lb is set
			if dns, ok := currentDNS[lb.ClusterInfo.Name]; ok {
				lbData.CurrentDNS = dns
			}
			newLbInfo.LbClusters = append(newLbInfo.LbClusters, lbData)
			//save new values
			lbInfos[lb.TargetedK8S] = newLbInfo
		} else {
			log.Error().Msgf("Loadbalancer %s from project %s has not found a target k8s cluster (%s)", lb.ClusterInfo.Name, req.DesiredState.Name, lb.TargetedK8S)
		}
	}
	if err := setUpLoadbalancers(lbInfos); err != nil {
		log.Error().Msgf("Error encountered while setting up the loadbalancers for project %s : %s", req.DesiredState.Name, err.Error())
		return nil, fmt.Errorf("error encountered while setting up the loadbalancers for project %s : %s", req.DesiredState.Name, err.Error())
	}
	log.Info().Msgf("Loadbalancers for project %s were successfully set up", req.DesiredState.Name)
	return &pb.SetUpLBResponse{DesiredState: req.DesiredState}, nil
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
			err = errors.New("ansibler interrupt signal")
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
