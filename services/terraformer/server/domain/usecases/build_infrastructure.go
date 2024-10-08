package usecases

import (
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
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
		SpawnProcessLimit:  u.SpawnProcessLimit,
	}

	var lbClusters []*loadbalancer.LBcluster
	for _, desiredLBCluster := range request.DesiredLbs {
		var current *spec.LBcluster

		for _, currentLbCluster := range request.CurrentLbs {
			if desiredLBCluster.ClusterInfo.Name == currentLbCluster.ClusterInfo.Name {
				current = currentLbCluster
				break
			}
		}

		lbClusters = append(lbClusters, &loadbalancer.LBcluster{
			DesiredState:      desiredLBCluster,
			CurrentState:      current,
			ProjectName:       request.ProjectName,
			SpawnProcessLimit: u.SpawnProcessLimit,
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
			cluster.UpdateCurrentState()
			logger.Error().Msgf("Error encountered while building cluster: %s", err)
			failed[idx] = err
			return err
		}

		cluster.UpdateCurrentState()
		logger.Info().Msgf("Infrastructure successfully created for cluster")
		return nil
	})

	if err != nil {
		response := &pb.BuildInfrastructureResponse_Fail{
			Fail: &pb.BuildInfrastructureResponse_InfrastructureData{
				Desired: k8sCluster.DesiredState,
			},
		}

		for _, cluster := range lbClusters {
			response.Fail.DesiredLbs = append(response.Fail.DesiredLbs, cluster.DesiredState)
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
				Desired:    request.Desired,
				DesiredLbs: request.DesiredLbs,
			},
		},
	}
	return resp, nil
}
