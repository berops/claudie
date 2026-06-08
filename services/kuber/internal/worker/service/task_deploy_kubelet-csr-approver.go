package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/proto/pb/spec"
	kca "github.com/berops/claudie/services/kuber/internal/worker/service/internal/kubelet-csr-approver"
	"github.com/rs/zerolog"
)

func DeployKubeletCSRApprover(
	logger zerolog.Logger,
	projectName string,
	tracker Tracker,
) {
	logger.Info().Msg("Deploying CSR approver")

	var k8s *spec.K8Scluster

	switch do := tracker.Task.Do.(type) {
	case *spec.Task_Create:
		k8s = do.Create.K8S
	case *spec.Task_Update:
		k8s = do.Update.State.K8S
	default:
		logger.
			Warn().
			Msgf("Received task %T while wanting to deploy CSR approver, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	var (
		tempClusterID = fmt.Sprintf("%s-%s", k8s.ClusterInfo.Id(), hash.Create(hash.Length))
		clusterDir    = filepath.Join(OutputDir, tempClusterID)
	)

	if err := fileutils.CreateDirectory(clusterDir); err != nil {
		err := fmt.Errorf("error while creating directory %s when deploying kubelet-csr-approver : %w", clusterDir, err)
		logger.Err(err).Msg("Failed to create directory for templates")
		tracker.Diagnostics.Push(err)
		return
	}

	defer func() {
		if err := os.RemoveAll(clusterDir); err != nil {
			logger.Err(err).Msg("Failed to remove directory where templates were generated")
			return
		}
	}()

	view := kca.ClusterView{
		Id:             k8s.ClusterInfo.Id(),
		Name:           k8s.ClusterInfo.Name,
		Kubeconfig:     k8s.Kubeconfig,
		PrivateNetwork: k8s.Network,
	}

	// Deploy kubelet-csr-approver
	kubeletCSRApprover := kca.NewKubeletCSRApprover(projectName, clusterDir, view)
	if err := kubeletCSRApprover.DeployKubeletCSRApprover(); err != nil {
		err := fmt.Errorf("error while deploying kubelet-csr-approver for %s : %w", k8s.ClusterInfo.Id(), err)
		logger.Err(err).Msg("Failed to deploy kubelet-csr-approver")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Finished deploying CSR approver")
}
