package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb"
	kca "github.com/berops/claudie/services/kuber/server/domain/utils/kubelet-csr-approver"
)

func (u *Usecases) DeployKubeletCSRApprover(ctx context.Context, request *pb.DeployKubeletCSRApproverRequest) (*pb.DeployKubeletCSRApproverResponse, error) {
	clusterID := request.Cluster.ClusterInfo.Id()
	logger := loggerutils.WithClusterName(clusterID)

	// Create output dir
	tempClusterID := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, hash.Create(5))
	clusterDir := filepath.Join(outputDir, tempClusterID)
	if err := fileutils.CreateDirectory(clusterDir); err != nil {
		logger.Err(err).Msgf("Error while creating directory %s when deploying kubelet-csr-approver", clusterDir)
		return nil, fmt.Errorf("error while creating directory %s when deploying kubelet-csr-approver : %w", clusterDir, err)
	}

	// Deploy kubelet-csr-approver
	kubeletCSRApprover := kca.NewKubeletCSRApprover(request.ProjectName, request.Cluster, clusterDir)
	if err := kubeletCSRApprover.DeployKubeletCSRApprover(); err != nil {
		logger.Err(err).Msgf("Error while deploying kubelet-csr-approver for %s", clusterID)
		return nil, fmt.Errorf("error while deploying kubelet-csr-approver for %s : %w", clusterID, err)
	}

	logger.Info().Msgf("Kubelet-csr-approver successfully deployed for %s", clusterID)

	return &pb.DeployKubeletCSRApproverResponse{}, nil
}
