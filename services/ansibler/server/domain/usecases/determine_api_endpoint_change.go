package usecases

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/clusters"
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

func (u *Usecases) DetermineApiEndpointChange(_ context.Context, request *pb.DetermineApiEndpointChangeRequest) (*pb.DetermineApiEndpointChangeResponse, error) {
	if request.Current == nil {
		return &pb.DetermineApiEndpointChangeResponse{Current: request.Current, CurrentLbs: request.CurrentLbs}, nil
	}

	logger := loggerutils.WithProjectAndCluster(request.ProjectName, request.Current.ClusterInfo.Id())
	logger.Info().Msgf("Received request for determining api endpoint state change %s, current: %s, desired: %s",
		request.State.String(),
		request.CurrentEndpointId,
		request.DesiredEndpointId,
	)

	if err := determineApiChanges(logger, request, u.SpawnProcessLimit); err != nil {
		msg := fmt.Sprintf("Failed to move api endpoint with requested state change %s, current: %s, desired: %s",
			request.State.String(),
			request.CurrentEndpointId,
			request.DesiredEndpointId,
		)
		logger.Err(err).Msgf(msg)
		return nil, fmt.Errorf("%s:%w", msg, err)
	}

	logger.Info().Msgf("Sucessfully processed request for determining api endpoint state change %s, current: %s, desired: %s",
		request.State.String(),
		request.CurrentEndpointId,
		request.DesiredEndpointId,
	)
	return &pb.DetermineApiEndpointChangeResponse{Current: request.Current, CurrentLbs: request.CurrentLbs}, nil
}

func determineApiChanges(
	logger zerolog.Logger,
	request *pb.DetermineApiEndpointChangeRequest,
	processLimit *semaphore.Weighted,
) error {
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

	if request.ProxyEnvs == nil {
		request.ProxyEnvs = &spec.ProxyEnvs{}
	}

	return handleAPIEndpointChange(logger, request, clusterDirectory, processLimit)
}

func handleAPIEndpointChange(
	logger zerolog.Logger,
	request *pb.DetermineApiEndpointChangeRequest,
	outputDirectory string,
	processLimit *semaphore.Weighted,
) error {
	var oldEndpoint, newEndpoint string

	switch request.State {
	case spec.ApiEndpointChangeState_NoChange, spec.ApiEndpointChangeState_EndpointRenamed:
		return nil
	case spec.ApiEndpointChangeState_AttachingLoadBalancer:
		_, n := nodepools.FindApiEndpoint(request.Current.ClusterInfo.NodePools)
		if n == nil {
			return fmt.Errorf("failed to find APIEndpoint k8s node, couldn't update Api server endpoint")
		}

		lb := clusters.IndexLoadbalancerById(request.DesiredEndpointId, request.CurrentLbs)
		if lb < 0 {
			return fmt.Errorf("failed to find requested loadbalancer %s to move api endpoint to", request.DesiredEndpointId)
		}

		n.NodeType = spec.NodeType_master
		oldEndpoint = n.Public

		request.CurrentLbs[lb].UsedApiEndpoint = true
		newEndpoint = request.CurrentLbs[lb].Dns.Endpoint
	case spec.ApiEndpointChangeState_DetachingLoadBalancer:
		lb := clusters.IndexLoadbalancerById(request.CurrentEndpointId, request.CurrentLbs)
		if lb < 0 {
			return fmt.Errorf("failed to find requested loadbalancer %s from which to move the api endpoint from", request.CurrentEndpointId)
		}

		n := nodepools.FirstControlNode(request.Current.ClusterInfo.NodePools)
		if n == nil {
			return fmt.Errorf("failed to find node with type %s", spec.NodeType_master.String())
		}

		request.CurrentLbs[lb].UsedApiEndpoint = false
		oldEndpoint = request.CurrentLbs[lb].Dns.Endpoint

		n.NodeType = spec.NodeType_apiEndpoint
		newEndpoint = n.Public
	case spec.ApiEndpointChangeState_MoveEndpoint:
		lbc := clusters.IndexLoadbalancerById(request.CurrentEndpointId, request.CurrentLbs)
		lbd := clusters.IndexLoadbalancerById(request.DesiredEndpointId, request.CurrentLbs)

		if lbc < 0 {
			return fmt.Errorf("failed to find requested loadbalancer %s from which to move the api endpoint from", request.CurrentEndpointId)
		}

		if lbd < 0 {
			return fmt.Errorf("failed to find requested loadbalancer %s to which to move the api endpoint", request.DesiredEndpointId)
		}

		request.CurrentLbs[lbc].UsedApiEndpoint = false
		request.CurrentLbs[lbd].UsedApiEndpoint = true

		oldEndpoint = request.CurrentLbs[lbc].Dns.Endpoint
		newEndpoint = request.CurrentLbs[lbd].Dns.Endpoint
	}

	logger.Debug().Str("LB-cluster", request.Current.ClusterInfo.Id()).Msgf("Changing the API endpoint from %s to %s", oldEndpoint, newEndpoint)
	return utils.ChangeAPIEndpoint(request.Current.ClusterInfo.Name, oldEndpoint, newEndpoint, outputDirectory, request.ProxyEnvs, processLimit)
}
