package usecases

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/loggerutils"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/ansibler/server/utils"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

func (u *Usecases) TeardownApiEndpointLoadbalancer(_ context.Context, request *pb.TeardownRequest) (*pb.TeardownResponse, error) {
	if request.Current == nil || len(request.CurrentLbs) == 0 {
		return &pb.TeardownResponse{Current: request.Current, CurrentLbs: request.CurrentLbs}, nil
	}

	var currentEndpoint *spec.LBcluster
	for _, lb := range request.CurrentLbs {
		if lb.HasApiRole() {
			currentEndpoint = lb
			break
		}
	}

	if currentEndpoint == nil {
		return &pb.TeardownResponse{Current: request.Current, CurrentLbs: request.CurrentLbs}, nil
	}

	logger := loggerutils.WithProjectAndCluster(request.ProjectName, request.Current.ClusterInfo.Id())
	logger.Info().
		Msgf("Moving api endpoint from loadbalancer %s to control plane node %s from nodepool %s",
			currentEndpoint.ClusterInfo.Id(),
			request.Enpoint.Node,
			request.Enpoint.Nodepool,
		)

	if err := teardownLoadBalancers(logger, currentEndpoint, request, u.SpawnProcessLimit); err != nil {
		msg := fmt.Sprintf("failed to move api endpoint for cluster %s project %s from loadbalancer %s to node %s from nodepool %s",
			request.Current.ClusterInfo.Id(),
			request.ProjectName,
			currentEndpoint.ClusterInfo.Id(),
			request.Enpoint.Node,
			request.Enpoint.Nodepool,
		)
		logger.Err(err).Msgf(msg)
		return nil, fmt.Errorf("%s:%w", msg, err)
	}

	logger.Info().
		Msgf("Api endpoint successfuly moved from loadbalancer %s to control plane node %s from nodepool %s",
			currentEndpoint.ClusterInfo.Id(),
			request.Enpoint.Node,
			request.Enpoint.Nodepool,
		)
	return &pb.TeardownResponse{Current: request.Current, CurrentLbs: request.CurrentLbs}, nil
}

func teardownLoadBalancers(
	logger zerolog.Logger,
	currentEndpoint *spec.LBcluster,
	request *pb.TeardownRequest,
	processLimit *semaphore.Weighted,
) error {
	np := nodepools.FindByName(request.Enpoint.Nodepool, request.Current.ClusterInfo.NodePools)
	if np == nil {
		return fmt.Errorf("no nodepool %q found within current state", request.Enpoint.Nodepool)
	}

	var newEndpointNode *spec.Node
	for _, node := range np.Nodes {
		if node.Name == request.Enpoint.Node {
			newEndpointNode = node
			break
		}
	}
	if newEndpointNode == nil {
		return fmt.Errorf("no node %q within nodepool %q found in current state", request.Enpoint.Node, request.Enpoint.Nodepool)
	}

	clusterDirectory := filepath.Join(
		baseDirectory,
		outputDirectory,
		fmt.Sprintf("%s-%s-lbs", request.Current.ClusterInfo.Id(), hash.Create(hash.Length)),
	)

	defer func() {
		if err := os.RemoveAll(clusterDirectory); err != nil {
			logger.Err(err).Msgf("failed to cleanup clusterdir %s for teardown loadbalancers", clusterDirectory)
		}
	}()

	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	dyn := nodepools.Dynamic(request.Current.ClusterInfo.NodePools)
	stc := nodepools.Static(request.Current.ClusterInfo.NodePools)

	if err := nodepools.DynamicGenerateKeys(dyn, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for dynamic nodepools : %w", err)
	}

	if err := nodepools.StaticGenerateKeys(stc, clusterDirectory); err != nil {
		return fmt.Errorf("failed to create key file(s) for static nodes : %w", err)
	}

	err := utils.GenerateInventoryFile(templates.LoadbalancerInventoryTemplate, clusterDirectory, utils.LBInventoryFileParameters{
		K8sNodepools: utils.NodePools{
			Dynamic: dyn,
			Static:  stc,
		},
		LBClusters: nil,
		ClusterID:  request.Current.ClusterInfo.Id(),
	})
	if err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", clusterDirectory, err)
	}

	oldEndpoint := currentEndpoint.Dns.Endpoint
	newEndpoint := newEndpointNode.Public
	newEndpointNode.NodeType = spec.NodeType_apiEndpoint

	// TODO: @checkin
	if request.ProxyEnvs == nil {
		request.ProxyEnvs = &spec.ProxyEnvs{}
	}

	return utils.ChangeAPIEndpoint(
		request.Current.ClusterInfo.Id(),
		oldEndpoint, newEndpoint,
		clusterDirectory,
		request.ProxyEnvs,
		processLimit,
	)
}
