package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/proto/pb"
)

func (u *Usecases) UpdateNoProxyEnvs(request *pb.UpdateNoProxyEnvsRequest) (*pb.UpdateNoProxyEnvsResponse, error) {
	if request.Current == nil {
		return &pb.UpdateNoProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
	}

	log.Info().Msgf("Updating NO_PROXY and no_proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
		request.Current.ClusterInfo.Name, request.ProjectName)
	if err := updateNoProxyEnvs(request.Current.ClusterInfo, request.Desired.ClusterInfo, u.SpawnProcessLimit); err != nil {
		return nil, fmt.Errorf("Failed to update NO_PROXY and no_proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
			request.Current.ClusterInfo.Name, request.ProjectName)
	}
	log.Info().Msgf("Updated NO_PROXY and no_proxy env variables in kube-proxy DaemonSet and static pods for cluster %s project %s",
		request.Current.ClusterInfo.Name, request.ProjectName)

	return &pb.UpdateNoProxyEnvsResponse{Current: request.Current, Desired: request.Desired}, nil
}

// updateNoProxyEnvs handles the case where Claudie adds/removes node to/from the cluster.
// Public and private IPs of this node must be added to the NO_PROXY and no_proxy env variables in
// kube-proxy DaemonSet and static pods.
func updateNoProxyEnvs(currentK8sClusterInfo, desiredK8sClusterInfo *pb.ClusterInfo, spawnProcessLimit chan struct{}) error {
	return nil
}
