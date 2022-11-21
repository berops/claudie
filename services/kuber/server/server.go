package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Berops/claudie/internal/envs"
	"github.com/Berops/claudie/internal/healthcheck"
	"github.com/Berops/claudie/internal/kubectl"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	"github.com/Berops/claudie/services/kuber/server/longhorn"
	"github.com/Berops/claudie/services/kuber/server/nodes"
	"github.com/Berops/claudie/services/kuber/server/secret"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	defaultKuberPort = 50057
	outputDir        = "services/kuber/server/clusters"
)

type server struct {
	pb.UnimplementedKuberServiceServer
}

func (s *server) SetUpStorage(ctx context.Context, req *pb.SetUpStorageRequest) (*pb.SetUpStorageResponse, error) {
	desiredState := req.GetDesiredState()
	var errGroup errgroup.Group
	for _, cluster := range desiredState.GetClusters() {
		func(c *pb.K8Scluster) {
			errGroup.Go(func() error {
				clusterID := fmt.Sprintf("%s-%s", c.ClusterInfo.Name, c.ClusterInfo.Hash)
				clusterDir := filepath.Join(outputDir, clusterID)
				longhorn := longhorn.Longhorn{Cluster: c, Directory: clusterDir}
				err := longhorn.SetUp()
				if err != nil {
					return fmt.Errorf("error while setting up the longhorn for %s : %w", clusterID, err)
				}
				log.Info().Msgf("Longhorn successfully set up on the cluster %s", clusterID)
				return nil
			})
		}(cluster)
	}
	if err := errGroup.Wait(); err != nil {
		log.Error().Msgf("Error encountered in SetUpStorage : %s", err.Error())
		return &pb.SetUpStorageResponse{DesiredState: desiredState, ErrorMessage: fmt.Sprintf("Error encountered in SetUpStorage: %s", err.Error())}, err
	}
	log.Info().Msgf("Storage was successfully set up for project %s", desiredState.Name)
	return &pb.SetUpStorageResponse{DesiredState: desiredState, ErrorMessage: ""}, nil
}

func (s *server) StoreKubeconfig(ctx context.Context, req *pb.StoreKubeconfigRequest) (*pb.StoreKubeconfigResponse, error) {
	cluster := req.GetCluster()
	var errGroup errgroup.Group
	func(c *pb.K8Scluster) {
		errGroup.Go(func() error {
			clusterID := fmt.Sprintf("%s-%s", c.ClusterInfo.Name, c.ClusterInfo.Hash)
			clusterDir := filepath.Join(outputDir, clusterID)

			if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
				if err := os.Mkdir(clusterDir, os.ModePerm); err != nil {
					return fmt.Errorf("Could not create a directory for %s", c.ClusterInfo.Name)
				}
			}
			sec := secret.New()
			sec.Directory = clusterDir
			// save kubeconfig as base64 encoded string
			sec.YamlManifest.Data.SecretData = base64.StdEncoding.EncodeToString([]byte(c.GetKubeconfig()))
			sec.YamlManifest.Metadata.Name = fmt.Sprintf("%s-kubeconfig", clusterID)
			namespace := envs.Namespace
			if namespace == "" {
				// the claudie is in local deployment - print kubeconfig
				log.Info().Msgf("The kubeconfig for %s:", clusterID)
				fmt.Println(c.Kubeconfig)
				//print and clean-up
				err := os.RemoveAll(sec.Directory)
				if err != nil {
					return fmt.Errorf("error while cleaning up the temporary directory %s : %w", sec.Directory, err)
				}
				return nil
			}
			// apply secret
			err := sec.Apply(namespace, "")
			if err != nil {
				return fmt.Errorf("error while creating the kubeconfig secret for %s", c.ClusterInfo.Name)
			}
			return nil
		})
	}(cluster)
	if err := errGroup.Wait(); err != nil {
		log.Error().Msgf("Error encountered in StoreKubeconfig : %s", err.Error())
		return &pb.StoreKubeconfigResponse{ErrorMessage: fmt.Sprintf("Error encountered in StoreKubeconfig : %s", err.Error())}, err
	}
	log.Info().Msgf("Kubeconfig was successfully stored for cluster %s", cluster.ClusterInfo.Name)
	return &pb.StoreKubeconfigResponse{ErrorMessage: ""}, nil
}

func (s *server) DeleteKubeconfig(ctx context.Context, req *pb.DeleteKubeconfigRequest) (*pb.DeleteKubeconfigResponse, error) {
	cluster := req.Cluster
	var errGroup errgroup.Group
	func(c *pb.K8Scluster) {
		errGroup.Go(func() error {
			secretName := fmt.Sprintf("%s-%s-kubeconfig", c.ClusterInfo.Name, c.ClusterInfo.Hash)
			namespace := envs.Namespace
			if namespace == "" {
				// the claudie is in local deployment - no action needed
				return nil
			}
			kc := kubectl.Kubectl{}

			// delete kubeconfig secret
			err := kc.KubectlDeleteResource("secret", secretName, namespace)
			if err != nil {
				return fmt.Errorf("error while deleting kubeconfig secret for %s : %w", c.ClusterInfo.Name, err)
			}
			return nil
		})
	}(cluster)
	if err := errGroup.Wait(); err != nil {
		log.Error().Msgf("Error encountered in DeleteKubeconfig for cluster %s : %s", cluster.ClusterInfo.Name, err.Error())
		return &pb.DeleteKubeconfigResponse{ErrorMessage: fmt.Sprintf("Error encountered in DeleteKubeconfig for cluster %s : %s", cluster.ClusterInfo.Name, err.Error())}, err
	}
	log.Info().Msgf("Deleted kubeconfig secret for cluster %s", cluster.ClusterInfo.Name)
	return &pb.DeleteKubeconfigResponse{ErrorMessage: ""}, nil
}

func (s *server) DeleteNodes(ctx context.Context, req *pb.DeleteNodesRequest) (*pb.DeleteNodesResponse, error) {
	deleter := nodes.New(req.MasterNodes, req.WorkerNodes, req.Cluster)
	cluster, err := deleter.DeleteNodes()
	if err != nil {
		log.Error().Msgf("Error while deleting nodes for %s : %s", req.Cluster.ClusterInfo.Name, err.Error())
		return &pb.DeleteNodesResponse{ErrorMessage: err.Error()}, err
	}
	log.Info().Msgf("Nodes for cluster %s were successfully deleted", req.Cluster.ClusterInfo.Name)
	return &pb.DeleteNodesResponse{ErrorMessage: "", Cluster: cluster}, nil
}

func main() {
	// initialize logger
	utils.InitLog("kuber")

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
	healthService := healthcheck.NewServerHealthChecker(kuberPort, "KUBER_PORT", nil)
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, ctx := errgroup.WithContext(context.Background())

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
			err = errors.New("kuber interrupt signal")
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

	g.Go(func() error {
		// s.Serve() will create a service goroutine for each connection
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("kuber failed to serve: %w", err)
		}
		log.Info().Msg("Finished listening for incoming connections")
		return nil
	})

	log.Info().Msgf("Stopping Kuber: %v", g.Wait())
}
