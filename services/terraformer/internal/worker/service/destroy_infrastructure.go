package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	cluster_builder "github.com/berops/claudie/services/terraformer/internal/worker/service/internal/cluster-builder"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/berops/claudie/services/terraformer/internal/worker/store"
	"github.com/rs/zerolog"
)

const (
	keyFormatStateFile    = "%s/%s"
	dnsKeyFormatStateFile = "%s/%s-dns"

	keyFormatLockFile    = "%s/%s/%s-md5"
	dnsKeyFormatLockFile = "%s/%s/%s-dns-md5"
)

// Destroys the infrastructure of the passed in [Cluster] by looking
// at its current state. On success update the [Cluster.UpdateCurrentState]
// to the desired state, which in this case will be [nil].
func DestroyCluster(
	logger zerolog.Logger,
	projectName string,
	cluster Cluster,
	s3 store.S3StateStorage,
	dynamo store.DynamoDB,
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

	// After the infrastructure is destroyed, we need to delete the tofu state file from MinIO and tofu state-lock file from DynamoDB.
	if err := dynamo.DeleteLockFile(ctx, projectName, cluster.Id(), keyFormatLockFile); err != nil {
		logger.Warn().Msgf("Failed to delete lock file, assumming it was deleted/not created")
	}
	if err := s3.DeleteStateFile(ctx, projectName, cluster.Id(), keyFormatStateFile); err != nil {
		logger.Warn().Msgf("Failed to delete state file, assumming it was deleted/not created")
	}
	logger.Info().Msgf("Successfully deleted tofu state and state-lock files")

	if err := os.RemoveAll(filepath.Join(cluster_builder.TemplatesRootDir, cluster.Id())); err != nil {
		return fmt.Errorf("failed to delete templates for cluster %q: %w", cluster.Id(), err)
	}
	logger.Info().Msgf("Successfully deleted Templates files for cluster")

	// In case of LoadBalancer type cluster,
	// there are additional DNS related tofu state and state-lock files.
	if _, ok := cluster.(*loadbalancer.LBcluster); ok {
		if err := dynamo.DeleteLockFile(ctx, projectName, cluster.Id(), dnsKeyFormatLockFile); err != nil {
			logger.Warn().Msgf("Failed to delete lock file for %q-dns, assumming it was deleted/not created", cluster.Id())
		}
		if err := s3.DeleteStateFile(ctx, projectName, cluster.Id(), dnsKeyFormatStateFile); err != nil {
			logger.Warn().Msgf("Failed to delete state file for %q-dns, assumming it was deleted/not created", cluster.Id())
		}
		logger.Info().Msg("Successfully deleted DNS related tofu state and state-lock files")

		if err := os.RemoveAll(filepath.Join(loadbalancer.TemplatesRootDir, fmt.Sprintf("%s-dns", cluster.Id()))); err != nil {
			return fmt.Errorf("failed to delete dns templates for cluster %q: %w", cluster.Id(), err)
		}
		logger.Info().Msgf("Successfully deleted dns Templates files for cluster %q", cluster.Id())
	}

	logger.Info().Msg("Cluster successfully destroyed")
	cluster.UpdateCurrentState()
	return nil
}
