package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb"
	kcr "github.com/berops/claudie/services/kuber/server/domain/utils/kubelet-csr-approver"
)

func (u *Usecases) DeployKubeletCSRApprover(ctx context.Context, request *pb.DeployKubeletCSRApproverRequest) (*pb.DeployKubeletCSRApproverResponse, error) {
	clusterID := request.Cluster.ClusterInfo.Id()
	logger := loggerutils.WithClusterName(clusterID)
	var err error
	// Log success/error message.
	defer func() {
		if err != nil {
			logger.Err(err).Msgf("Error while deploying kubelet-csr-approver")
		} else {
			logger.Info().Msgf("kubelet-csr-approver successfully deployed")
		}
	}()

	// Create output dir
	tempClusterID := fmt.Sprintf("%s-%s", request.Cluster.ClusterInfo.Name, hash.Create(5))
	clusterDir := filepath.Join(outputDir, tempClusterID)
	if err := fileutils.CreateDirectory(clusterDir); err != nil {
		return nil, fmt.Errorf("error while creating directory %s : %w", clusterDir, err)
	}

	// Deploy kubelet-csr-approver
	kubeletCSRApprover := kcr.NewKubeletCSRApprover(request.ProjectName, request.Cluster, clusterDir)
	if err := kubeletCSRApprover.DeployKubeletCSRApprover(); err != nil {
		return nil, fmt.Errorf("error while deploying kubelet-csr-approver for %s : %w", clusterID, err)
	}
	return &pb.DeployKubeletCSRApproverResponse{}, nil
}
