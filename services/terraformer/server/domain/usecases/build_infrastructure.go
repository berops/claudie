package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/kubernetes"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/loadbalancer"
)

// BuildInfrastructure builds the required infrastructure for a single Kubernetes cluster
// and the Loadbalancer clusters related to it
func (u *Usecases) BuildInfrastructure(request *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	// clusters slice contains those Kubernetes and Loadbalancer clusters
	// for which infrastructure will be built.
	// The "clusters" slice is initialized with the desired Kubernetes cluster we want to build.
	clusters := []Cluster{
		kubernetes.K8Scluster{
			ProjectName: request.ProjectName,

			DesiredState: request.Desired,
			CurrentState: request.Current,

			AttachedLBClusters: request.DesiredLbs,
		},
	}

	// LB clusters which appear in both request.CurrentLbClusters and request.DesiredLBClusters
	// are appended to the "clusters" slice
	for _, desiredLBCluster := range request.DesiredLbs {
		var commonLb *pb.LBcluster

		for _, currentLbCluster := range request.CurrentLbs {
			if desiredLBCluster.ClusterInfo.Name == currentLbCluster.ClusterInfo.Name {
				commonLb = currentLbCluster
				break
			}
		}

		clusters = append(clusters,
			loadbalancer.LBcluster{
				DesiredState: desiredLBCluster,
				CurrentState: commonLb,

				ProjectName: request.ProjectName,
			},
		)
	}

	// Concurrently build infrastructure for each cluster in the "clusters" slice
	err := utils.ConcurrentExec(clusters, func(cluster Cluster) error {
		logger := utils.CreateLoggerWithProjectAndClusterName(request.ProjectName, cluster.Id())
		logger.Info().Msg("Creating infrastructure")

		if err := cluster.Build(logger); err != nil {
			return fmt.Errorf("error while building the cluster %v : %w", cluster.Id(), err)
		}

		logger.Info().Msgf("Infrastructure successfully created for cluster")
		return nil
	})
	if err != nil {
		log.Err(err).Str("project", request.ProjectName).Msgf("Error encountered while building cluster")
		return nil, fmt.Errorf("error while building cluster %s for project %s : %w", request.Desired.ClusterInfo.Name, request.ProjectName, err)
	}

	response := &pb.BuildInfrastructureResponse{
		Current: request.Current,
		Desired: request.Desired,

		CurrentLbs: request.CurrentLbs,
		DesiredLbs: request.DesiredLbs,
	}

	return response, nil
}
