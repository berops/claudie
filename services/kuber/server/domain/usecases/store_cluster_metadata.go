package usecases

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/secret"
)

// StoreClusterMetadata constructs ClusterMetadata for the given K8s cluster, creates a Kubernetes
// secret out of that and stores that secret in the Claudie management cluster.
func (u *Usecases) StoreClusterMetadata(ctx context.Context, request *pb.StoreClusterMetadataRequest) (*pb.StoreClusterMetadataResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.Cluster.ClusterInfo))

	// Construct ClusterMetadata for the given cluster.
	clusterMetadata := ClusterMetadata{
		NodeIps:    make(map[string]IPPair),
		PrivateKey: request.GetCluster().GetClusterInfo().GetPrivateKey(),
	}
	for _, nodePool := range request.GetCluster().GetClusterInfo().GetNodePools() {
		for _, node := range nodePool.GetNodes() {

			clusterMetadata.NodeIps[node.Name] = IPPair{
				PublicIP:  net.ParseIP(node.Public),
				PrivateIP: net.ParseIP(node.Private),
			}
		}
	}

	b, err := json.Marshal(clusterMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s cluster metadata: %w", request.GetCluster().GetClusterInfo().GetName(), err)
	}

	// TODO: understand (this most probably handles the case when Claudie is deployed outside Kubernetes)
	// local deployment - print metadata
	if namespace := envs.Namespace; namespace == "" {
		// NOTE: DEBUG print
		// var buffer bytes.Buffer
		// for node, ips := range clusterMetadata.NodeIps {
		// 	buffer.WriteString(fmt.Sprintf("%s: %v \t| %v \n", node, ips.PublicIP, ips.PrivateIP))
		// }
		// buffer.WriteString(fmt.Sprintf("%s\n", md.PrivateKey))
		// log.Info().Msgf("Cluster metadata from cluster %s \n%s", request.GetCluster().ClusterInfo.Name, buffer.String())
		return &pb.StoreClusterMetadataResponse{}, nil
	}

	logger.Info().Msgf("Storing cluster metadata")

	clusterID := fmt.Sprintf("%s-%s", request.GetCluster().ClusterInfo.Name, request.GetCluster().ClusterInfo.Hash)
	outputDir := filepath.Join(outputDir, clusterID)

	k8sSecret := secret.New(outputDir,
		secret.NewYaml(
			secret.Metadata{Name: fmt.Sprintf("%s-metadata", clusterID)},
			map[string]string{"metadata": base64.StdEncoding.EncodeToString(b)},
		),
	)

	// TODO: understand (most probably the K8s secret is created in the management cluster itself where
	// Claudie is running).
	// TODO: understand what is meant by default kubeconfig in k8sSecret.Apply.
	if err := k8sSecret.Apply(envs.Namespace, ""); err != nil {
		logger.Err(err).Msgf("Failed to store cluster metadata")
		return nil, fmt.Errorf("error while creating cluster metadata secret for %s", request.Cluster.ClusterInfo.Name)
	}

	logger.Info().Msgf("Cluster metadata was successfully stored")

	return &pb.StoreClusterMetadataResponse{}, nil
}
