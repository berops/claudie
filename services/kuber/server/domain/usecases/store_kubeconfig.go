package usecases

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/envs"
	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/kuber/server/domain/utils"
	"github.com/berops/claudie/services/kuber/server/domain/utils/secret"
)

func (u *Usecases) StoreKubeconfig(ctx context.Context, request *pb.StoreKubeconfigRequest) (*pb.StoreKubeconfigResponse, error) {
	cluster := request.GetCluster()
	clusterID := cutils.GetClusterID(request.Cluster.ClusterInfo)
	logger := cutils.CreateLoggerWithClusterName(clusterID)

	if envs.Namespace == "" {
		//NOTE: DEBUG print
		// logger.Info().Msgf("The kubeconfig for %s\n%s:", clusterID, cluster.Kubeconfig)
		return &pb.StoreKubeconfigResponse{}, nil
	}

	logger.Info().Msgf("Storing kubeconfig")
	clusterDir := filepath.Join(outputDir, clusterID)
	sec := secret.New(clusterDir, secret.NewYaml(
		utils.GetSecretMetadata(request.Cluster.ClusterInfo, request.ProjectName, utils.KubeconfigSecret),
		map[string]string{"kubeconfig": base64.StdEncoding.EncodeToString([]byte(cluster.GetKubeconfig()))},
	))

	if err := sec.Apply(envs.Namespace, ""); err != nil {
		logger.Err(err).Msgf("Failed to store kubeconfig")
		return nil, fmt.Errorf("error while creating the kubeconfig secret for %s", cluster.ClusterInfo.Name)
	}

	logger.Info().Msgf("Kubeconfig was successfully stored")
	return &pb.StoreKubeconfigResponse{}, nil
}
