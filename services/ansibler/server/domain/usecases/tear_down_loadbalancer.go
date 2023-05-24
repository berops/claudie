package usecases

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	commonUtils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/ansibler/server/utils"
)

// TeardownLoadBalancers correctly destroys loadbalancers by selecting the new ApiServer endpoint.
func (a *Usecases) TeardownLoadBalancers(ctx context.Context, request *pb.TeardownLBRequest) (*pb.TeardownLBResponse, error) {
	// If no LB clusters were deleted from the manifest, then just return.
	if len(request.DeletedLbs) == 0 {
		return &pb.TeardownLBResponse{
			PreviousAPIEndpoint: "",
			Desired:             request.Desired,
			DesiredLbs:          request.DesiredLbs,
			DeletedLbs:          request.DeletedLbs,
		}, nil
	}

	logger := log.With().Str("project", request.ProjectName).Str("cluster", request.Desired.ClusterInfo.Name).Logger()
	logger.Info().Msgf("Tearing down the loadbalancers")

	var isApiServerTypeDesiredLBClusterPresent bool
	for _, lbCluster := range request.DesiredLbs {
		if commonUtils.HasAPIServerRole(lbCluster.Roles) {
			isApiServerTypeDesiredLBClusterPresent = true
		}
	}

	// For each load-balancer that is being deleted construct LbClusterData.
	lbClustersInfo := &utils.LBClustersInfo{
		ClusterID: fmt.Sprintf("%s-%s", request.Desired.ClusterInfo.Name, request.Desired.ClusterInfo.Hash),

		TargetK8sNodepool:    request.Desired.ClusterInfo.NodePools,
		TargetK8sNodepoolKey: request.Desired.ClusterInfo.PrivateKey,
	}
	for _, lbCluster := range request.DeletedLbs {
		lbClustersInfo.LbClusters = append(lbClustersInfo.LbClusters, &utils.LBClusterData{
			DesiredLbCluster: nil,
			CurrentLbCluster: lbCluster,
		})
	}

	previousApiEndpoint, err := teardownLoadBalancers(request.Desired.ClusterInfo.Name, lbClustersInfo, isApiServerTypeDesiredLBClusterPresent)
	if err != nil {
		logger.Err(err).Msgf("Error encountered while tearing down the LoadBalancers")
		return nil, fmt.Errorf("error encountered while tearing down loadbalancers for cluster %s project %s : %w", request.Desired.ClusterInfo.Name, request.ProjectName, err)
	}

	logger.Info().Msgf("Loadbalancers were successfully torn down")
	response := &pb.TeardownLBResponse{
		PreviousAPIEndpoint: previousApiEndpoint,
		Desired:             request.Desired,
		DesiredLbs:          request.DesiredLbs,
		DeletedLbs:          request.DeletedLbs,
	}
	return response, nil
}

// tearDownLoadBalancers will correctly destroy LB clusters (including correctly selecting the new ApiServer if present).
// If for a K8s cluster a new ApiServerLB is being attached instead of handling the apiEndpoint immediately
// it will be delayed and will send the data to the dataChan which will be used later for the SetupLoadbalancers
// function to bypass generating the certificates for the endpoint multiple times.
func teardownLoadBalancers(clusterName string, lbClustersInfo *utils.LBClustersInfo, attached bool) (string, error) {
	outputDirectory := filepath.Join(baseDirectory, outputDirectory, fmt.Sprintf("%s-%s-lbs", clusterName, commonUtils.CreateHash(commonUtils.HashLength)))
	if err := utils.GenerateLBBaseFiles(outputDirectory, lbClustersInfo); err != nil {
		return "", fmt.Errorf("error encountered while generating base files for %s", clusterName)
	}

	currentApiServerTypeLBCluster := utils.FindCurrentAPIServerTypeLBCluster(lbClustersInfo.LbClusters)
	// If there is an Api server type LB cluster currently that will be deleted, and we're attaching a
	// new Api server type LB cluster to the K8s cluster, we store the endpoint being used by the
	// current Api server type LB cluster.
	// This will be reused later in the SetUpLoadbalancers function.
	if currentApiServerTypeLBCluster != nil && attached {
		return currentApiServerTypeLBCluster.CurrentLbCluster.Dns.Endpoint, os.RemoveAll(outputDirectory)
	}

	if err := utils.HandleAPIEndpointChange(currentApiServerTypeLBCluster, lbClustersInfo, outputDirectory); err != nil {
		return "", err
	}

	return "", os.RemoveAll(outputDirectory)
}
