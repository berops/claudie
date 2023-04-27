package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	internalUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	utils "github.com/berops/claudie/services/context-box/server/utils"
)

func updateNodepool(state *pb.Project, clusterName, nodepoolName string, nodes []*pb.Node, count *int32) error {
	for _, cluster := range state.Clusters {
		if cluster.ClusterInfo.Name == clusterName {
			for _, nodepool := range cluster.ClusterInfo.NodePools {
				if nodepool.Name == nodepoolName {
					// Update nodes
					nodepool.Nodes = nodes
					if count != nil {
						nodepool.Count = *count
					}
					return nil
				}
			}
			return fmt.Errorf("nodepool %s was not found in cluster %s", nodepoolName, clusterName)
		}
	}
	return fmt.Errorf("cluster %s was not found in project %s", clusterName, state.Name)
}

func (u *Usecases) UpdateNodepool(request *pb.UpdateNodepoolRequest) (*pb.UpdateNodepoolResponse, error) {
	// Input specification can be changed on two places, by Autoscaler and by User, thus we need to lock it, so one does not overwrite the other.
	u.configChangeMutex.Lock()
	defer u.configChangeMutex.Unlock()
	log.Info().Msgf("CLIENT REQUEST: UpdateNodepoolCount for Project %s, Cluster %s Nodepool %s", request.ProjectName, request.ClusterName, request.Nodepool.Name)
	var config *pb.Config
	var err error
	if config, err = u.DB.GetConfig(request.ProjectName, pb.IdType_NAME); err != nil {
		return nil, fmt.Errorf("the project %s was not found in the database : %w ", request.ProjectName, err)
	}
	// Check if config is currently not in any build stage or in a queue
	if config.BuilderTTL == 0 && config.SchedulerTTL == 0 && !u.schedulerQueue.Contains(config) && !u.builderQueue.Contains(config) {
		// Check if all checksums are equal, meaning config is not about to get pushed to the queue or is in the queue
		if utils.CompareChecksum(config.MsChecksum, config.DsChecksum) && utils.CompareChecksum(config.DsChecksum, config.CsChecksum) {
			// Find and update correct nodepool count & nodes in desired state.
			if err := updateNodepool(config.DesiredState, request.ClusterName, request.Nodepool.Name, request.Nodepool.Nodes, &request.Nodepool.Count); err != nil {
				return nil, fmt.Errorf("error while updating desired state in project %s : %w", config.Name, err)
			}
			// Find and update correct nodepool nodes in current state.
			// This has to be done in order
			if err := updateNodepool(config.CurrentState, request.ClusterName, request.Nodepool.Name, request.Nodepool.Nodes, nil); err != nil {
				return nil, fmt.Errorf("error while updating current state in project %s : %w", config.Name, err)
			}
			// Save new config in the database with dummy CsChecksum to initiate a build.
			config.CsChecksum = utils.CalculateChecksum(internalUtils.CreateHash(8))
			if err := u.DB.SaveConfig(config); err != nil {
				return nil, err
			}
			return &pb.UpdateNodepoolResponse{}, nil
		}
		return nil, fmt.Errorf("the project %s is about to be in the build stage", request.ProjectName)
	}
	return nil, fmt.Errorf("the project %s is currently in the build stage", request.ProjectName)
}
