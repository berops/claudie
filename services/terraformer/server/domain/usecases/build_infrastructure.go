package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/kubernetes"
	"github.com/berops/claudie/services/terraformer/server/loadbalancer"
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

			DesiredK8s: request.Desired,
			CurrentK8s: request.Current,

			LoadBalancers: request.DesiredLbs,
		},
	}

	// LB clusters which appear in both request.CurrentLbs and request.DesiredLbs
	// are appended to the "clusters" slice
	for _, desiredLb := range request.DesiredLbs {
		var commonLb *pb.LBcluster

		for _, currentLb := range request.CurrentLbs {
			if desiredLb.ClusterInfo.Name == currentLb.ClusterInfo.Name {
				commonLb = currentLb
				break
			}
		}

		clusters = append(clusters, loadbalancer.LBcluster{DesiredLB: desiredLb, CurrentLB: commonLb, ProjectName: request.ProjectName})
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
		Current: request.Current,
		Desired: request.Desired,

		CurrentLbs: request.CurrentLbs,
		DesiredLbs: request.DesiredLbs,
	}

	return response, nil
}
