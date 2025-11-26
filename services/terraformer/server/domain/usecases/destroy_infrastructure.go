package usecases

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/concurrent"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/proto/pb"
	outboundAdapters "github.com/berops/claudie/services/terraformer/server/adapters/outbound"
	cluster_builder "github.com/berops/claudie/services/terraformer/server/domain/utils/cluster-builder"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/kubernetes"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/loadbalancer"
	"github.com/rs/zerolog/log"
)

const (
	keyFormatStateFile    = "%s/%s"
	dnsKeyFormatStateFile = "%s/%s-dns"

	keyFormatLockFile    = "%s/%s/%s-md5"
	dnsKeyFormatLockFile = "%s/%s/%s-dns-md5"
)

// DestroyInfrastructure destroys the infrastructure for provided LB clusters
// and a Kubernetes cluster (if provided).
func (u *Usecases) DestroyInfrastructure(ctx context.Context, request *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	var clusters []Cluster

	// If infrastructure for a Kuberenetes cluster needs to be destroyed
	// then add the Kubernetes cluster to the "clusters" slice.
	if request.Current != nil {
		clusters = append(clusters, &kubernetes.K8Scluster{
			ProjectName:       request.ProjectName,
			CurrentState:      request.Current,
			SpawnProcessLimit: u.SpawnProcessLimit,
		})
	}

	for _, currentLB := range request.CurrentLbs {
		clusters = append(clusters, &loadbalancer.LBcluster{
			ProjectName:       request.ProjectName,
			CurrentState:      currentLB,
			SpawnProcessLimit: u.SpawnProcessLimit,
		})
	}

	// Concurrently destroy the infrastructure, tofu state and state-lock files for each cluster
	err := concurrent.Exec(clusters, func(_ int, cluster Cluster) error {
		logger := loggerutils.WithProjectAndCluster(request.ProjectName, cluster.Id())
		err := u.StateStorage.Stat(ctx, request.ProjectName, cluster.Id(), keyFormatStateFile)
		if err != nil {
			if errors.Is(err, outboundAdapters.ErrKeyNotExists) {
				logger.Warn().Msgf("no state file found for cluster %q, assuming the infrastructure was deleted.", cluster.Id())
				return nil
			}
			return fmt.Errorf("failed to check existence of state file for %q: %w", cluster.Id(), err)
		}
		logger.Debug().Msgf("infrastructure state file present for cluster %q", cluster.Id())

		logger.Info().Msgf("Destroying infrastructure")

		if err := cluster.Destroy(logger); err != nil {
			return fmt.Errorf("error while destroying cluster %v : %w", cluster.Id(), err)
		}
		logger.Info().Msgf("Infrastructure was successfully destroyed")

		// After the infrastructure is destroyed, we need to delete the tofu state file from MinIO.
		if err := u.StateStorage.DeleteStateFile(ctx, request.ProjectName, cluster.Id(), keyFormatStateFile); err != nil {
			logger.Warn().Msgf("Failed to delete state file for %q, assumming it was deleted/not created", cluster.Id())
		}
		logger.Info().Msgf("Successfully deleted tofu state and state-lock files for %q", cluster.Id())

		if err := os.RemoveAll(filepath.Join(cluster_builder.TemplatesRootDir, cluster.Id())); err != nil {
			return fmt.Errorf("failed to delete templates for cluster %q: %w", cluster.Id(), err)
		}
		logger.Info().Msgf("Successfully deleted Templates files for cluster %q", cluster.Id())

		// In case of LoadBalancer type cluster,
		// there are additional DNS related tofu state.
		if _, ok := cluster.(*loadbalancer.LBcluster); ok {
			if err := u.StateStorage.DeleteStateFile(ctx, request.ProjectName, cluster.Id(), dnsKeyFormatStateFile); err != nil {
				logger.Warn().Msgf("Failed to delete state file for %q-dns, assumming it was deleted/not created", cluster.Id())
			}
			logger.Info().Msg("Successfully deleted DNS related tofu state and state-lock files")

			if err := os.RemoveAll(filepath.Join(loadbalancer.TemplatesRootDir, fmt.Sprintf("%s-dns", cluster.Id()))); err != nil {
				return fmt.Errorf("failed to delete dns templates for cluster %q: %w", cluster.Id(), err)
			}
			logger.Info().Msgf("Successfully deleted dns Templates files for cluster %q", cluster.Id())
		}

		return nil
	})

	if err != nil {
		log.Error().Msgf("Error while destroying the infrastructure for project %s : %s", request.ProjectName, err)
		return nil, fmt.Errorf("error while destroying infrastructure for project %s : %w", request.ProjectName, err)
	}

	response := &pb.DestroyInfrastructureResponse{
		Current:    request.Current,
		CurrentLbs: request.CurrentLbs,
	}

	return response, nil
}
