package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/autoscaler"
	"github.com/berops/claudie/services/kuber/server/nodes"
	"github.com/berops/claudie/services/kuber/server/secret"
)

const (
	outputDir = "services/kuber/server/clusters"
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

func (s *server) DeleteClusterMetadata(ctx context.Context, req *pb.DeleteClusterMetadataRequest) (*pb.DeleteClusterMetadataResponse, error) {
	namespace := envs.Namespace
	if namespace == "" {
		return &pb.DeleteClusterMetadataResponse{}, nil
	}

	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(req.Cluster.ClusterInfo))
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
	clusterID := utils.GetClusterID(req.Cluster.ClusterInfo)
	logger := utils.CreateLoggerWithClusterName(clusterID)
	if namespace := envs.Namespace; namespace == "" {
		//NOTE: DEBUG print
		// logger.Info().Msgf("The kubeconfig for %s\n%s:", clusterID, cluster.Kubeconfig)
		return &pb.StoreKubeconfigResponse{}, nil
	}

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
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(req.Cluster.ClusterInfo))

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
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(req.Cluster.ClusterInfo))

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
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(req.Cluster.ClusterInfo))

	patcher := nodes.NewPatcher(req.Cluster)
	if err := patcher.PatchProviderID(logger); err != nil {
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

	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(req.Cluster.ClusterInfo))

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

	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(req.Cluster.ClusterInfo))

	// Destroy cluster autoscaler.
	autoscalerBuilder := autoscaler.NewAutoscalerBuilder(req.ProjectName, req.Cluster, clusterDir)
	if err := autoscalerBuilder.DestroyClusterAutoscaler(); err != nil {
		logger.Err(err).Msgf("Error while destroying cluster autoscaler")
		return nil, fmt.Errorf("error while destroying cluster autoscaler for %s : %w", req.Cluster.ClusterInfo.Name, err)
	}

	logger.Info().Msgf("Cluster autoscaler successfully destroyed")
	return &pb.DestroyClusterAutoscalerResponse{}, nil
}
