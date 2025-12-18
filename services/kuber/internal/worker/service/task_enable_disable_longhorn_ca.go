package service

import (
	"fmt"
	"os"
	"path/filepath"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/kuber/templates"
	"github.com/rs/zerolog"
)

type enableCA struct {
	IsAutoscaled string
}

func EnableLonghornCA(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Enabling cluster autoscaler support for longhorn")

	var k8s *spec.K8SclusterV2

	switch do := tracker.Task.Do.(type) {
	case *spec.TaskV2_Create:
		k8s = do.Create.K8S
	case *spec.TaskV2_Update:
		k8s = do.Update.State.K8S
	default:
		logger.
			Warn().
			Msgf("Received task %T while wanting to setup storage, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	var (
		tempClusterId = fmt.Sprintf("%s-%s", k8s.ClusterInfo.Id(), hash.Create(7))
		clusterDir    = filepath.Join(OutputDir, tempClusterId)
		template      = templateUtils.Templates{
			Directory: clusterDir,
		}
	)

	if err := fileutils.CreateDirectory(clusterDir); err != nil {
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

	tmpl, err := templateUtils.LoadTemplate(templates.EnableClusterAutoscalerTemplate)
	if err != nil {
		logger.Err(err).Msg("Failed to build Cluster Autoscaler settings template")
		tracker.Diagnostics.Push(err)
		return
	}

	ca := enableCA{
		IsAutoscaled: fmt.Sprint(true),
	}

	setting, err := template.GenerateToString(tmpl, ca)
	if err != nil {
		logger.Err(err).Msg("Failed to generate Cluster Autoscaler settings template")
		tracker.Diagnostics.Push(err)
		return
	}

	k := kubectl.Kubectl{
		Kubeconfig:        k8s.Kubeconfig,
		MaxKubectlRetries: 3,
	}
	k.Stdout = comm.GetStdOut(k8s.ClusterInfo.Id())
	k.Stderr = comm.GetStdErr(k8s.ClusterInfo.Id())

	if err := k.KubectlApplyString(setting); err != nil {
		logger.Err(err).Msg("Failed to apply Cluster Autoscaler settings template")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Cluster autoscaler support for longhorn, enabled")
}

func DisableLonghornCA(logger zerolog.Logger, tracker Tracker) {
	logger.Info().Msg("Disabling cluster autoscaler support for longhorn")

	var k8s *spec.K8SclusterV2

	switch do := tracker.Task.Do.(type) {
	case *spec.TaskV2_Create:
		k8s = do.Create.K8S
	case *spec.TaskV2_Update:
		k8s = do.Update.State.K8S
	default:
		logger.
			Warn().
			Msgf("Received task %T while wanting to setup storage, assuming it was mischeduled, ignoring", tracker.Task.Do)
		return
	}

	var (
		tempClusterId = fmt.Sprintf("%s-%s", k8s.ClusterInfo.Id(), hash.Create(7))
		clusterDir    = filepath.Join(OutputDir, tempClusterId)
		template      = templateUtils.Templates{
			Directory: clusterDir,
		}
	)

	if err := fileutils.CreateDirectory(clusterDir); err != nil {
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

	tmpl, err := templateUtils.LoadTemplate(templates.EnableClusterAutoscalerTemplate)
	if err != nil {
		logger.Err(err).Msg("Failed to build Cluster Autoscaler settings template")
		tracker.Diagnostics.Push(err)
		return
	}

	ca := enableCA{
		IsAutoscaled: fmt.Sprint(false),
	}

	setting, err := template.GenerateToString(tmpl, ca)
	if err != nil {
		logger.Err(err).Msg("Failed to generate Cluster Autoscaler settings template")
		tracker.Diagnostics.Push(err)
		return
	}

	k := kubectl.Kubectl{
		Kubeconfig:        k8s.Kubeconfig,
		MaxKubectlRetries: 3,
	}
	k.Stdout = comm.GetStdOut(k8s.ClusterInfo.Id())
	k.Stderr = comm.GetStdErr(k8s.ClusterInfo.Id())

	if err := k.KubectlApplyString(setting); err != nil {
		logger.Err(err).Msg("Failed to apply Cluster Autoscaler settings template")
		tracker.Diagnostics.Push(err)
		return
	}

	logger.Info().Msg("Cluster autoscaler support for longhorn, disabled")
}
