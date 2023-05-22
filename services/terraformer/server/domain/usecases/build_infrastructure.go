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
		log.Info().Msgf("Creating infrastructure for cluster %s project %s", cluster.Id(), request.ProjectName)

		if err := cluster.Build(); err != nil {
			return fmt.Errorf("error while building the cluster %v : %w", cluster.Id(), err)
		}

		log.Info().Msgf("Infrastructure was successfully created for cluster %s project %s", cluster.Id(), request.ProjectName)
		return nil
	})
	if err != nil {
		log.Error().Msgf("Error while building cluster %s for project %s : %s", request.Desired.ClusterInfo.Name, request.ProjectName, err)
		return nil, fmt.Errorf("error while building cluster %s for project %s : %w", request.Desired.ClusterInfo.Name, request.ProjectName, err)
	}

	response := &pb.BuildInfrastructureResponse{
		//  related to the K8s cluster
		Current: request.Current,
		Desired: request.Desired,

		CurrentLbs: request.CurrentLbs,
		DesiredLbs: request.DesiredLbs,
	}

	return response, nil
}
