package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/concurrent"
	"github.com/berops/claudie/proto/pb/spec"
	cluster_builder "github.com/berops/claudie/services/terraformer/internal/worker/service/internal/cluster-builder"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/berops/claudie/services/terraformer/internal/worker/store"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

const (
	keyFormatStateFile    = "%s/%s"
	dnsKeyFormatStateFile = "%s/%s-dns"
)

func destroy(
	logger zerolog.Logger,
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Destroying infrastructure")

	var clusters []Cluster

	action, ok := tracker.Task.Do.(*spec.Task_Delete)
	if !ok {
		logger.
			Warn().
			Msgf("received task with action %T while wanting to destroy infrastructure, assuming the task was misscheduled, ignoring", tracker.Task.Do)
		return
	}

	k8s, loadbalancers := action.Delete.K8S, action.Delete.LoadBalancers
	if k8s == nil {
		logger.
			Warn().
			Msg("delete task validation failed, required kubernetes state to be present, but is missing, ignoring")
		return
	}

	clusters = append(clusters, &kubernetes.K8Scluster{
		ProjectName:       projectName,
		Cluster:           k8s,
		SpawnProcessLimit: processLimit,
	})

	for _, lb := range loadbalancers {
		if lb == nil {
			logger.
				Warn().
				Msg("delete task validation failed, required loadbalancer state to be present, but is missing, ignoring")
			return
		}

		clusters = append(clusters, &loadbalancer.LBcluster{
			ProjectName:       projectName,
			Cluster:           lb,
			SpawnProcessLimit: processLimit,
		})
	}

	ids := make([]string, len(clusters))
	errs := make([]error, len(clusters))

	err := concurrent.Exec(clusters, func(idx int, cluster Cluster) error {
		buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()
		ids[idx] = cluster.Id()
		errs[idx] = DestroyCluster(buildLogger, projectName, cluster, stores.s3)
		return errs[idx]
	})
	if err != nil {
		logger.Err(err).Msg("Failed to destroy clusters")
		// Some of the provided clusters didn't destroy successfully.
		// Since we still want to report the partially destroyed infrastructure
		// back to the caller we fallthrough here, as any of the successfully destroyed
		// infrastructure will have its [CurrentState] updated to [nil].
		tracker.Diagnostics.Push(err)
	}

	var (
		k8sId string
		lbIds []string
	)

	for i, c := range clusters {
		if errs[i] == nil {
			if c.IsKubernetes() {
				k8sId = ids[i]
			} else {
				lbIds = append(lbIds, ids[i])
			}
		}
	}

	infraClear := tracker.Result.Clear()
	if k8sId != "" {
		infraClear.Kubernetes()
	}
	infraClear.LoadBalancers(lbIds...)
	infraClear.Commit()
}

// Destroys the infrastructure of the passed in [Cluster] by looking
// at its current state. On success update the [Cluster.UpdateCurrentState]
// to the desired state, which in this case will be [nil].
func DestroyCluster(
	logger zerolog.Logger,
	projectName string,
	cluster Cluster,
	s3 store.S3StateStorage,
) error {
	logger.Info().Msg("Destroying infrastructure")

	ctx := context.Background()

	if err := s3.Stat(ctx, projectName, cluster.Id(), keyFormatStateFile); err != nil {
		if errors.Is(err, store.ErrS3KeyNotExists) {
			logger.Warn().Msgf("no state file found for cluster, assuming the infrastructure was deleted")
			return nil
		}
		return fmt.Errorf("failed to check existence of state file for %q: %w", cluster.Id(), err)
	}

	logger.Debug().Msgf("infrastructure state file present for cluster")
	logger.Info().Msgf("Destroying infrastructure")

	if err := cluster.Destroy(logger); err != nil {
		return fmt.Errorf("error while destroying cluster %v : %w", cluster.Id(), err)
	}

	logger.Info().Msgf("Infrastructure was successfully destroyed")

	// After the infrastructure is destroyed, we need to delete the tofu state file from MinIO.
	if err := s3.DeleteStateFile(ctx, projectName, cluster.Id(), keyFormatStateFile); err != nil {
		logger.Warn().Msgf("Failed to delete state file, assumming it was deleted/not created")
	}
	logger.Info().Msgf("Successfully deleted tofu state and state-lock files")

	if err := os.RemoveAll(filepath.Join(cluster_builder.TemplatesRootDir, cluster.Id())); err != nil {
		return fmt.Errorf("failed to delete templates for cluster %q: %w", cluster.Id(), err)
	}
	logger.Info().Msgf("Successfully deleted Templates files for cluster")

	// In case of LoadBalancer type cluster,
	// there is additional DNS related tofu state.
	if _, ok := cluster.(*loadbalancer.LBcluster); ok {
		if err := s3.DeleteStateFile(ctx, projectName, cluster.Id(), dnsKeyFormatStateFile); err != nil {
			logger.Warn().Msgf("Failed to delete state file for %q-dns, assumming it was deleted/not created", cluster.Id())
		}
		logger.Info().Msg("Successfully deleted DNS related tofu state and state-lock files")

		if err := os.RemoveAll(filepath.Join(loadbalancer.TemplatesRootDir, fmt.Sprintf("%s-dns", cluster.Id()))); err != nil {
			return fmt.Errorf("failed to delete dns templates for cluster %q: %w", cluster.Id(), err)
		}
		logger.Info().Msgf("Successfully deleted dns Templates files for cluster %q", cluster.Id())
	}

	logger.Info().Msg("Processed destroyed infrastructure")
	return nil
}
