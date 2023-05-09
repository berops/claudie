package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/autoscaler"
	"github.com/berops/claudie/services/kuber/server/longhorn"
	"github.com/berops/claudie/services/kuber/server/nodes"
	scrapeconfig "github.com/berops/claudie/services/kuber/server/scrapeConfig"
	"github.com/berops/claudie/services/kuber/server/secret"
)

const (
	defaultKuberPort = 50057
	outputDir        = "services/kuber/server/clusters"
)

type (
	IPPair struct {
		PublicIP  net.IP `json:"public_ip"`
		PrivateIP net.IP `json:"private_ip"`
	}

	ClusterMetadata struct {
		// NodeIps maps node-name to public-private ip pairs.
		NodeIps map[string]IPPair `json:"node_ips"`
		// PrivateKey is the private SSH key for the nodes.
		PrivateKey string `json:"private_key"`
	}
)

type server struct {
	pb.UnimplementedKuberServiceServer
}

func (s *server) SetUpStorage(ctx context.Context, req *pb.SetUpStorageRequest) (*pb.SetUpStorageResponse, error) {
	logger := log.With().Str("cluster", req.DesiredCluster.ClusterInfo.Name).Logger()

	clusterID := fmt.Sprintf("%s-%s", req.DesiredCluster.ClusterInfo.Name, req.DesiredCluster.ClusterInfo.Hash)
	clusterDir := filepath.Join(outputDir, clusterID)

	logger.Info().Msgf("Setting up the longhorn")
	longhorn := longhorn.Longhorn{Cluster: req.DesiredCluster, Directory: clusterDir}
	if err := longhorn.SetUp(); err != nil {
		return nil, fmt.Errorf("error while setting up the longhorn for %s : %w", clusterID, err)
	}
	logger.Info().Msgf("Longhorn successfully set up")

	return &pb.SetUpStorageResponse{DesiredCluster: req.DesiredCluster}, nil
}

func (s *server) StoreLbScrapeConfig(ctx context.Context, req *pb.StoreLbScrapeConfigRequest) (*pb.StoreLbScrapeConfigResponse, error) {
	logger := log.With().Str("cluster", req.Cluster.ClusterInfo.Name).Logger()

	clusterID := fmt.Sprintf("%s-%s", req.Cluster.ClusterInfo.Name, req.Cluster.ClusterInfo.Hash)
	clusterDir := filepath.Join(outputDir, clusterID)
	logger.Info().Msgf("Storing load balancer scrape-config")

	sc := scrapeconfig.ScrapeConfig{
		Cluster:    req.GetCluster(),
		LBClusters: req.GetDesiredLoadbalancers(),
		Directory:  clusterDir,
	}

	if err := sc.GenerateAndApplyScrapeConfig(); err != nil {
		return nil, fmt.Errorf("error while setting up the loadbalancer scrape-config for %s : %w", clusterID, err)
	}
	logger.Info().Msgf("Load balancer scrape-config successfully set up")

	return &pb.StoreLbScrapeConfigResponse{}, nil
}

func (s *server) RemoveLbScrapeConfig(ctx context.Context, req *pb.RemoveLbScrapeConfigRequest) (*pb.RemoveLbScrapeConfigResponse, error) {
	logger := log.With().Str("cluster", req.Cluster.ClusterInfo.Name).Logger()

	clusterID := fmt.Sprintf("%s-%s", req.Cluster.ClusterInfo.Name, req.Cluster.ClusterInfo.Hash)
	clusterDir := filepath.Join(outputDir, clusterID)
	logger.Info().Msgf("Deleting load balancer scrape-config")

	sc := scrapeconfig.ScrapeConfig{
		Cluster:   req.GetCluster(),
		Directory: clusterDir,
	}

	if err := sc.RemoveLbScrapeConfig(); err != nil {
		return nil, fmt.Errorf("error while removing old loadbalancer scrape-config for %s : %w", clusterID, err)
	}
	logger.Info().Msgf("Load balancer scrape-config successfully deleted")

	return &pb.RemoveLbScrapeConfigResponse{}, nil
}

func (s *server) StoreClusterMetadata(ctx context.Context, req *pb.StoreClusterMetadataRequest) (*pb.StoreClusterMetadataResponse, error) {
	logger := log.With().Str("cluster", req.Cluster.ClusterInfo.Name).Logger()

	md := ClusterMetadata{
		NodeIps:    make(map[string]IPPair),
		PrivateKey: req.GetCluster().GetClusterInfo().GetPrivateKey(),
	}

	for _, pool := range req.GetCluster().GetClusterInfo().GetNodePools() {
		for _, node := range pool.GetNodes() {
			md.NodeIps[node.Name] = IPPair{
				PublicIP:  net.ParseIP(node.Public),
				PrivateIP: net.ParseIP(node.Private),
			}
		}
	}

	b, err := json.Marshal(md)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s cluster metadata: %w", req.GetCluster().GetClusterInfo().GetName(), err)
	}

	// local deployment - print metadata
	if namespace := envs.Namespace; namespace == "" {
		// NOTE: DEBUG print
		// var buffer bytes.Buffer
		// for node, ips := range md.NodeIps {
		// 	buffer.WriteString(fmt.Sprintf("%s: %v \t| %v \n", node, ips.PublicIP, ips.PrivateIP))
		// }
		// buffer.WriteString(fmt.Sprintf("%s\n", md.PrivateKey))
		// log.Info().Msgf("Cluster metadata from cluster %s \n%s", req.GetCluster().ClusterInfo.Name, buffer.String())
		return &pb.StoreClusterMetadataResponse{}, nil
	}
	logger.Info().Msgf("Storing cluster metadata")

	clusterID := fmt.Sprintf("%s-%s", req.GetCluster().ClusterInfo.Name, req.GetCluster().ClusterInfo.Hash)
	clusterDir := filepath.Join(outputDir, clusterID)
	sec := secret.New(clusterDir, secret.NewYaml(
		secret.Metadata{Name: fmt.Sprintf("%s-metadata", clusterID)},
		map[string]string{"metadata": base64.StdEncoding.EncodeToString(b)},
	))

	if err := sec.Apply(envs.Namespace, ""); err != nil {
		logger.Err(err).Msgf("Failed to store cluster metadata")
		return nil, fmt.Errorf("error while creating cluster metadata secret for %s", req.Cluster.ClusterInfo.Name)
	}

	logger.Info().Msgf("Cluster metadata was successfully stored")
	return &pb.StoreClusterMetadataResponse{}, nil
}

func (s *server) DeleteClusterMetadata(ctx context.Context, req *pb.DeleteClusterMetadataRequest) (*pb.DeleteClusterMetadataResponse, error) {
	namespace := envs.Namespace
	if namespace == "" {
		return &pb.DeleteClusterMetadataResponse{}, nil
	}

	logger := log.With().Str("cluster", req.Cluster.ClusterInfo.Name).Logger()

	logger.Info().Msgf("Deleting cluster metadata secret")

	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", req.Cluster.ClusterInfo.Name, req.Cluster.ClusterInfo.Hash)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	secretName := fmt.Sprintf("%s-%s-metadata", req.Cluster.ClusterInfo.Name, req.Cluster.ClusterInfo.Hash)
	if err := kc.KubectlDeleteResource("secret", secretName, "-n", namespace); err != nil {
		logger.Warn().Msgf("Failed to remove cluster metadata: %s", err)
		return &pb.DeleteClusterMetadataResponse{}, nil
	}

	logger.Info().Msgf("Deleted cluster metadata secret")
	return &pb.DeleteClusterMetadataResponse{}, nil
}

func (s *server) StoreKubeconfig(ctx context.Context, req *pb.StoreKubeconfigRequest) (*pb.StoreKubeconfigResponse, error) {
	// local deployment - print kubeconfig
	cluster := req.GetCluster()
	clusterID := fmt.Sprintf("%s-%s", cluster.ClusterInfo.Name, cluster.ClusterInfo.Hash)
	if namespace := envs.Namespace; namespace == "" {
		//NOTE: DEBUG print
		// log.Info().Msgf("The kubeconfig for %s\n%s:", clusterID, cluster.Kubeconfig)
		return &pb.StoreKubeconfigResponse{}, nil
	}

	logger := log.With().Str("cluster", req.Cluster.ClusterInfo.Name).Logger()

	logger.Info().Msgf("Storing kubeconfig")

	clusterDir := filepath.Join(outputDir, clusterID)
	sec := secret.New(clusterDir, secret.NewYaml(
		secret.Metadata{Name: fmt.Sprintf("%s-kubeconfig", clusterID)},
		map[string]string{"kubeconfig": base64.StdEncoding.EncodeToString([]byte(cluster.GetKubeconfig()))},
	))

	if err := sec.Apply(envs.Namespace, ""); err != nil {
		logger.Err(err).Msgf("Failed to store kubeconfig")
		return nil, fmt.Errorf("error while creating the kubeconfig secret for %s", cluster.ClusterInfo.Name)
	}

	logger.Info().Msgf("Kubeconfig was successfully stored")
	return &pb.StoreKubeconfigResponse{}, nil
}

func (s *server) DeleteKubeconfig(ctx context.Context, req *pb.DeleteKubeconfigRequest) (*pb.DeleteKubeconfigResponse, error) {
	namespace := envs.Namespace
	if namespace == "" {
		return &pb.DeleteKubeconfigResponse{}, nil
	}
	cluster := req.GetCluster()

	logger := log.With().Str("cluster", req.Cluster.ClusterInfo.Name).Logger()

	logger.Info().Msgf("Deleting kubeconfig secret")
	kc := kubectl.Kubectl{MaxKubectlRetries: 3}
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		prefix := fmt.Sprintf("%s-%s", req.Cluster.ClusterInfo.Name, req.Cluster.ClusterInfo.Hash)
		kc.Stdout = comm.GetStdOut(prefix)
		kc.Stderr = comm.GetStdErr(prefix)
	}
	secretName := fmt.Sprintf("%s-%s-kubeconfig", cluster.ClusterInfo.Name, cluster.ClusterInfo.Hash)

	if err := kc.KubectlDeleteResource("secret", secretName, "-n", namespace); err != nil {
		logger.Warn().Msgf("Failed to remove kubeconfig: %s", err)
		return &pb.DeleteKubeconfigResponse{}, nil
	}

	logger.Info().Msgf("Deleted kubeconfig secret")
	return &pb.DeleteKubeconfigResponse{}, nil
}

func (s *server) DeleteNodes(ctx context.Context, req *pb.DeleteNodesRequest) (*pb.DeleteNodesResponse, error) {
	logger := log.With().Str("cluster", req.Cluster.ClusterInfo.Name).Logger()

	logger.Info().Msgf("Deleting nodes - control nodes [%d], compute nodes[%d]", len(req.MasterNodes), len(req.WorkerNodes))
	deleter := nodes.NewDeleter(req.MasterNodes, req.WorkerNodes, req.Cluster)
	cluster, err := deleter.DeleteNodes()
	if err != nil {
		logger.Err(err).Msgf("Error while deleting nodes")
		return &pb.DeleteNodesResponse{}, err
	}
	logger.Info().Msgf("Nodes were successfully deleted")
	return &pb.DeleteNodesResponse{Cluster: cluster}, nil
}

func (s *server) PatchNodes(ctx context.Context, req *pb.PatchNodeTemplateRequest) (*pb.PatchNodeTemplateResponse, error) {
	logger := log.With().Str("cluster", req.Cluster.ClusterInfo.Name).Logger()

	patcher := nodes.NewPatcher(req.Cluster)
	if err := patcher.PatchProviderID(); err != nil {
		logger.Err(err).Msgf("Error while patching nodes")
		return nil, fmt.Errorf("error while patching nodes for %s : %w", req.Cluster.ClusterInfo.Name, err)
	}

	logger.Info().Msgf("Nodes were successfully patched")
	return &pb.PatchNodeTemplateResponse{}, nil
}

func (s *server) SetUpClusterAutoscaler(ctx context.Context, req *pb.SetUpClusterAutoscalerRequest) (*pb.SetUpClusterAutoscalerResponse, error) {
	// Create output dir
	clusterID := fmt.Sprintf("%s-%s", req.Cluster.ClusterInfo.Name, utils.CreateHash(5))
	clusterDir := filepath.Join(outputDir, clusterID)
	if err := utils.CreateDirectory(clusterDir); err != nil {
		return nil, fmt.Errorf("error while creating directory %s : %w", clusterDir, err)
	}

	logger := log.With().Str("cluster", req.Cluster.ClusterInfo.Name).Logger()

	// Set up cluster autoscaler.
	autoscalerBuilder := autoscaler.NewAutoscalerBuilder(req.ProjectName, req.Cluster, clusterDir)
	if err := autoscalerBuilder.SetUpClusterAutoscaler(); err != nil {
		logger.Err(err).Msgf("Error while setting up cluster autoscaler")
		return nil, fmt.Errorf("error while setting up cluster autoscaler for %s : %w", req.Cluster.ClusterInfo.Name, err)
	}

	logger.Info().Msgf("Cluster autoscaler successfully set up")
	return &pb.SetUpClusterAutoscalerResponse{}, nil
}

func (s *server) DestroyClusterAutoscaler(ctx context.Context, req *pb.DestroyClusterAutoscalerRequest) (*pb.DestroyClusterAutoscalerResponse, error) {
	// Create output dir
	clusterID := fmt.Sprintf("%s-%s", req.Cluster.ClusterInfo.Name, utils.CreateHash(5))
	clusterDir := filepath.Join(outputDir, clusterID)
	if err := utils.CreateDirectory(clusterDir); err != nil {
		return nil, fmt.Errorf("error while creating directory %s : %w", clusterDir, err)
	}

	logger := log.With().Str("cluster", req.Cluster.ClusterInfo.Name).Logger()

	// Destroy cluster autoscaler.
	autoscalerBuilder := autoscaler.NewAutoscalerBuilder(req.ProjectName, req.Cluster, clusterDir)
	if err := autoscalerBuilder.DestroyClusterAutoscaler(); err != nil {
		logger.Err(err).Msgf("Error while destroying cluster autoscaler")
		return nil, fmt.Errorf("error while destroying cluster autoscaler for %s : %w", req.Cluster.ClusterInfo.Name, err)
	}

	logger.Info().Msgf("Cluster autoscaler successfully destroyed")
	return &pb.DestroyClusterAutoscalerResponse{}, nil
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
	healthServer := health.NewServer()
	// Kuber does not have any custom health check functions, thus always serving.
	healthServer.SetServingStatus("kuber-liveness", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("kuber-readiness", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthServer)

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
