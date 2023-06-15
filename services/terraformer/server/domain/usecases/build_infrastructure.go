package usecases

import (
	"errors"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/kubernetes"
	"github.com/berops/claudie/services/terraformer/server/domain/utils/loadbalancer"
)

// BuildInfrastructure builds the required infrastructure for a single Kubernetes cluster
// and the Loadbalancer clusters related to it
func (u *Usecases) BuildInfrastructure(request *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	k8sCluster := &kubernetes.K8Scluster{
		ProjectName:        request.ProjectName,
		DesiredState:       request.Desired,
		CurrentState:       request.Current,
		AttachedLBClusters: request.DesiredLbs,
	}

	var lbClusters []*loadbalancer.LBcluster
	for _, desiredLBCluster := range request.DesiredLbs {
		var current *pb.LBcluster

		for _, currentLbCluster := range request.CurrentLbs {
			if desiredLBCluster.ClusterInfo.Name == currentLbCluster.ClusterInfo.Name {
				current = currentLbCluster
				break
			}
		}

		lbClusters = append(lbClusters, &loadbalancer.LBcluster{
			DesiredState: desiredLBCluster,
			CurrentState: current,
			ProjectName:  request.ProjectName,
		})
	}

	clusters := []Cluster{k8sCluster}
	for _, lb := range lbClusters {
		clusters = append(clusters, lb)
	}

	failed := make([]error, len(clusters))
	err := utils.ConcurrentExec(clusters, func(idx int, cluster Cluster) error {
		logger := utils.CreateLoggerWithProjectAndClusterName(request.ProjectName, cluster.Id())
		logger.Info().Msg("Creating infrastructure")

		if err := cluster.Build(logger); err != nil {
			if errors.Is(err, loadbalancer.ErrCreateDNSRecord) {
				cluster.UpdateCurrentState()
			}
			logger.Err(err).Msgf("Error encountered while building cluster")
			failed[idx] = err
			return err
		}

		cluster.UpdateCurrentState()
		logger.Info().Msgf("Infrastructure successfully created for cluster")
		return nil
	})

	if err != nil {
		// Given a failure in the building process construct the response such that those that successfully
		// build will have their current state = desired state.
		// This is done as to when the calling side might want to delete the infra, it has its current state.
		response := &pb.BuildInfrastructureResponse_Fail{
			Fail: &pb.BuildInfrastructureResponse_InfrastructureData{
				Current: k8sCluster.CurrentState,
				Desired: k8sCluster.DesiredState,
			},
		}

		for _, cluster := range lbClusters {
			response.Fail.DesiredLbs = append(response.Fail.CurrentLbs, cluster.DesiredState)
			if cluster.CurrentState != nil {
				response.Fail.CurrentLbs = append(response.Fail.CurrentLbs, cluster.CurrentState)
			}
		}

		// add unchanged clusters that were not in lbClusters
		for _, cluster := range request.CurrentLbs {
			var found bool

			for _, buildCluster := range lbClusters {
				if cluster.ClusterInfo.Name == buildCluster.CurrentState.ClusterInfo.Name {
					found = true
					break
				}
			}

			if !found {
				response.Fail.CurrentLbs = append(response.Fail.CurrentLbs, cluster)
			}
		}

		for idx, err := range failed {
			if err != nil {
				response.Fail.Failed = append(response.Fail.Failed, clusters[idx].Id())
			}
		}

		return &pb.BuildInfrastructureResponse{Response: response}, nil
	}

	// with no errors we don't set the current state = desired state as it can be easily done from the calling side.
	resp := &pb.BuildInfrastructureResponse{
		Response: &pb.BuildInfrastructureResponse_Ok{
			Ok: &pb.BuildInfrastructureResponse_InfrastructureData{
				Current:    request.Current,
				Desired:    request.Desired,
				CurrentLbs: request.CurrentLbs,
				DesiredLbs: request.DesiredLbs,
			},
		},
	}
	return resp, nil
}
