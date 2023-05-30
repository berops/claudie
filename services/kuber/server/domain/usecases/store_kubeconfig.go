package usecases

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/secret"
)

// StoreKubeconfig creates a Kubernetes secret in the management K8s custer running Claudie.
// This secret contains kubeconfig for the given K8s cluster.
func (u *Usecases) StoreKubeconfig(ctx context.Context, request *pb.StoreKubeconfigRequest) (*pb.StoreKubeconfigResponse, error) {
	clusterID := utils.GetClusterID(request.Cluster.ClusterInfo)
	logger := utils.CreateLoggerWithClusterName(clusterID)

	k8sCluster := request.GetCluster()
	outputDir := filepath.Join(outputDir, clusterID)

	// If Claudie is running outside Kubernetes
	if namespace := envs.Namespace; namespace == "" {
		//NOTE: DEBUG print
		// logger.Info().Msgf("The kubeconfig for %s\n%s:", clusterID, k8sCluster.Kubeconfig)
		return &pb.StoreKubeconfigResponse{}, nil
	}

	logger.Info().Msgf("Storing kubeconfig")

	k8sSecret := secret.New(outputDir,
		secret.NewYaml(
			secret.Metadata{Name: fmt.Sprintf("%s-kubeconfig", clusterID)},
			map[string]string{"kubeconfig": base64.StdEncoding.EncodeToString([]byte(k8sCluster.GetKubeconfig()))},
		),
	)
	if err := k8sSecret.Apply(envs.Namespace, ""); err != nil {
		logger.Err(err).Msgf("Failed to store kubeconfig in management cluster")
		return nil, fmt.Errorf("error while creating the K8s secret storing kubeconfig of %s", k8sCluster.ClusterInfo.Name)
	}

	logger.Info().Msgf("Kubeconfig was successfully stored")
	return &pb.StoreKubeconfigResponse{}, nil
}
