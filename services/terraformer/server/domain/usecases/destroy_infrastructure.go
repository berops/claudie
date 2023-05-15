package usecases

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/kubernetes"
	"github.com/berops/claudie/services/terraformer/server/loadbalancer"
)

// DestroyInfrastructure destroys the infrastructure for provided LB clusters
// and a Kubernetes cluster (if provided).
func (u *Usecases) DestroyInfrastructure(ctx context.Context, request *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	var clusters []Cluster

	// If infrastructure for a Kuberenetes cluster needs to be destroyed
	// then add the Kubernetes cluster to the "clusters" slice.
	if request.Current != nil {
		clusters = append(clusters,
			kubernetes.K8Scluster{
				ProjectName:  request.ProjectName,
				CurrentState: request.Current,
			},
		)
	}

	for _, currentLB := range request.CurrentLbs {
		clusters = append(clusters,
			loadbalancer.LBcluster{
				ProjectName:  request.ProjectName,
				CurrentState: currentLB,
			},
		)
	}

	// Concurrently destroy the infrastructure, Terraform state and state-lock files for each cluster
	err := utils.ConcurrentExec(clusters, func(cluster Cluster) error {
		log.Info().Msgf("Destroying infrastructure for cluster %s project %s", cluster.Id(), request.ProjectName)

		if err := cluster.Destroy(); err != nil {
			return fmt.Errorf("error while destroying cluster %v : %w", cluster.Id(), err)
		}
		log.Info().Msgf("Infrastructure for cluster %s project %s was successfully destroyed", cluster.Id(), request.ProjectName)

		// After the infrastructure is destroyed, we need to delete the Terraform state file from MinIO
		// and Terraform state-lock file from DynamoDB.
		if err := u.DynamoDB.DeleteTfStateLockFile(ctx, request.ProjectName, cluster.Id(), false); err != nil {
			return err
		}
		if err := u.MinIO.DeleteTfStateFile(ctx, request.ProjectName, cluster.Id(), false); err != nil {
			return err
		}
		log.Info().Msgf("Successfully deleted Terraform state and state-lock files for cluster %s in project %s", cluster.Id(), request.ProjectName)

		// In case of LoadBalancer type cluster,
		// there are additional DNS related Terraform state and state-lock files.
		if _, ok := cluster.(loadbalancer.LBcluster); ok {
			if err := u.DynamoDB.DeleteTfStateLockFile(ctx, request.ProjectName, cluster.Id(), true); err != nil {
				return err
			}
			if err := u.MinIO.DeleteTfStateFile(ctx, request.ProjectName, cluster.Id(), true); err != nil {
				return err
			}
			log.Info().Msgf("Successfully deleted DNS related Terraform state and state-lock files for cluster %s in project %s", cluster.Id(), request.ProjectName)
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
