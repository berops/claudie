package usecases

import (
	"errors"

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

	k8slogger := utils.CreateLoggerWithProjectAndClusterName(request.ProjectName, k8sCluster.Id())
	k8slogger.Info().Msg("Creating infrastructure")
	if err := k8sCluster.Build(k8slogger); err != nil {
		k8slogger.Err(err).Msgf("failed tu build k8s cluster")
		return &pb.BuildInfrastructureResponse{
			Response: &pb.BuildInfrastructureResponse_Fail{
				Fail: &pb.BuildInfrastructureResponse_InfrastructureData{
					Desired:    k8sCluster.CurrentState,
					DesiredLbs: request.CurrentLbs,
					Failed:     []string{k8sCluster.Id()},
				},
			},
		}, nil
	}

	k8sCluster.UpdateCurrentState()
	k8slogger.Info().Msgf("Infrastructure successfully created for cluster")

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

	failed := make([]error, len(lbClusters))
	err := utils.ConcurrentExec(lbClusters, func(idx int, cluster *loadbalancer.LBcluster) error {
		logger := utils.CreateLoggerWithProjectAndClusterName(request.ProjectName, cluster.Id())
		logger.Info().Msg("Creating infrastructure")

		if err := cluster.Build(logger); err != nil {
			logger.Error().Msgf("Error encountered while building cluster: %s", err)
			failed[idx] = err

			if errors.Is(err, loadbalancer.ErrCreateNodePools) {
				return err
			}

			if errors.Is(err, loadbalancer.ErrCreateDNSRecord) {
				// infra build, dns failed, if there is an
				// existing current state keep it and do
				// not overwrite to desired state dns (which failed).
				var dns *spec.DNS
				if cluster.CurrentState != nil {
					dns = cluster.CurrentState.Dns
				}
				cluster.UpdateCurrentState()
				cluster.CurrentState.Dns = dns
				return err
			}

			return err
		}

		cluster.UpdateCurrentState()
		logger.Info().Msgf("Infrastructure successfully created for cluster")
		return nil
	})
	if err != nil {
		response := &pb.BuildInfrastructureResponse_Fail{
			Fail: &pb.BuildInfrastructureResponse_InfrastructureData{
				Desired: k8sCluster.CurrentState,
			},
		}

		for _, cluster := range lbClusters {
			if cluster.CurrentState != nil {
				response.Fail.DesiredLbs = append(response.Fail.DesiredLbs, cluster.CurrentState)
			}
		}

		for idx, err := range failed {
			if err != nil {
				response.Fail.Failed = append(response.Fail.Failed, lbClusters[idx].Id())
			}
		}

		return &pb.BuildInfrastructureResponse{Response: response}, nil
	}

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
