package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/fileutils"
	"github.com/berops/claudie/internal/hash"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	utils "github.com/berops/claudie/services/ansibler/internal/worker/service/internal"
	"github.com/berops/claudie/services/ansibler/templates"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

// MoveApiEndpoint moves the api endpoint to another node/loadbalancer within the specified cluster.
func MoveApiEndpoint(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Moving API endpoint")
	action, ok := tracker.Task.Do.(*spec.TaskV2_Update)
	if !ok {
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to move api endpoint, assuming the task was misscheduled, ignoring", tracker.Task.GetDo())
		return
	}

	var ep spec.UpdateV2_ApiEndpoint

	switch delta := action.Update.Delta.(type) {
	case *spec.UpdateV2_ApiEndpoint_:
		ep.State = delta.ApiEndpoint.State
		ep.CurrentEndpointId = delta.ApiEndpoint.CurrentEndpointId
		ep.DesiredEndpointId = delta.ApiEndpoint.DesiredEndpointId
	case *spec.UpdateV2_ReplacedDns_:
		if delta.ReplacedDns.OldApiEndpoint == nil {
			logger.
				Warn().
				Msgf("Received task with action %T, but with missing [OldApiEndpoint], assuming the task was misscheduled, ignoring", delta)
			return
		}

		idx := clusters.IndexLoadbalancerByIdV2(delta.ReplacedDns.Handle, action.Update.State.LoadBalancers)
		lb := action.Update.State.LoadBalancers[idx]

		ep.State = spec.ApiEndpointChangeStateV2_EndpointRenamedV2
		ep.CurrentEndpointId = *delta.ReplacedDns.OldApiEndpoint
		ep.DesiredEndpointId = lb.Dns.Endpoint // this should have been build by now.

		if ep.CurrentEndpointId == "" || ep.DesiredEndpointId == "" {
			logger.
				Warn().
				Msgf(
					"Received valid task for moving the api endpoint, but the required values are missing, "+
						"prev %q, new %q change %q, assuming the task was misscheduled, ignoring",
					ep.CurrentEndpointId,
					ep.DesiredEndpointId,
					ep.State.String(),
				)
			return
		}
	default:
		logger.
			Warn().
			Msgf("Received task with action %T while wanting to move api endpoint, assuming the task was misscheduled, ignoring", action.Update.Delta)
		return
	}

	if err := determineApiChanges(logger, action.Update.State, &ep, processLimit); err != nil {
		logger.Err(err).Msg("Failed to move Api endpoint")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Kubernetes(action.Update.State.K8S)
	update.Loadbalancers(action.Update.State.LoadBalancers...)
	update.Commit()

	logger.Info().Msg("Successfully moved API endpoint")
}

func determineApiChanges(
	logger zerolog.Logger,
	state *spec.UpdateV2_State,
	change *spec.UpdateV2_ApiEndpoint,
	processLimit *semaphore.Weighted,
) error {
	clusterDirectory := filepath.Join(
		BaseDirectory,
		OutputDirectory,
		fmt.Sprintf("%s-%s-lbs", state.K8S.ClusterInfo.Id(), hash.Create(hash.Length)),
	)

	defer func() {
		if err := os.RemoveAll(clusterDirectory); err != nil {
			logger.Err(err).Msgf("failed to cleanup clusterdir %s for teardown loadbalancers", clusterDirectory)
		}
	}()

	if err := fileutils.CreateDirectory(clusterDirectory); err != nil {
		return fmt.Errorf("failed to create directory %s : %w", clusterDirectory, err)
	}

	dyn := nodepools.Dynamic(state.K8S.ClusterInfo.NodePools)
	stc := nodepools.Static(state.K8S.ClusterInfo.NodePools)

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
		ClusterID:  state.K8S.ClusterInfo.Id(),
	})
	if err != nil {
		return fmt.Errorf("error while creating inventory file for %s : %w", clusterDirectory, err)
	}
	return handleAPIEndpointChange(logger, state, change, clusterDirectory, processLimit)
}

func handleAPIEndpointChange(
	logger zerolog.Logger,
	state *spec.UpdateV2_State,
	change *spec.UpdateV2_ApiEndpoint,
	outputDirectory string,
	processLimit *semaphore.Weighted,
) error {
	var oldEndpoint, newEndpoint string

	switch change.State {
	case spec.ApiEndpointChangeStateV2_NoChangeV2:
		return nil
	case spec.ApiEndpointChangeStateV2_EndpointRenamedV2:
		// There does not need to by any change in the [spec.LBcluster.UsedApiEndpoint]
		// as it's the same loadbalancer just the dns hostname has changed.
		oldEndpoint = change.CurrentEndpointId
		newEndpoint = change.DesiredEndpointId
	case spec.ApiEndpointChangeStateV2_AttachingLoadBalancerV2:
		_, n := nodepools.FindApiEndpoint(state.K8S.ClusterInfo.NodePools)
		if n == nil {
			return fmt.Errorf("failed to find APIEndpoint k8s node, couldn't update Api server endpoint")
		}

		lb := clusters.IndexLoadbalancerByIdV2(change.DesiredEndpointId, state.LoadBalancers)
		if lb < 0 {
			return fmt.Errorf("failed to find requested loadbalancer %s to move api endpoint to", change.DesiredEndpointId)
		}

		n.NodeType = spec.NodeType_master
		oldEndpoint = n.Public

		state.LoadBalancers[lb].UsedApiEndpoint = true
		newEndpoint = state.LoadBalancers[lb].Dns.Endpoint
	case spec.ApiEndpointChangeStateV2_DetachingLoadBalancerV2:
		lb := clusters.IndexLoadbalancerByIdV2(change.CurrentEndpointId, state.LoadBalancers)
		if lb < 0 {
			return fmt.Errorf("failed to find requested loadbalancer %s from which to move the api endpoint from", change.CurrentEndpointId)
		}

		n := nodepools.FirstControlNode(state.K8S.ClusterInfo.NodePools)
		if n == nil {
			return fmt.Errorf("failed to find node with type %s", spec.NodeType_master.String())
		}

		state.LoadBalancers[lb].UsedApiEndpoint = false
		oldEndpoint = state.LoadBalancers[lb].Dns.Endpoint

		n.NodeType = spec.NodeType_apiEndpoint
		newEndpoint = n.Public
	case spec.ApiEndpointChangeStateV2_MoveEndpointV2:
		lbc := clusters.IndexLoadbalancerByIdV2(change.CurrentEndpointId, state.LoadBalancers)
		lbd := clusters.IndexLoadbalancerByIdV2(change.DesiredEndpointId, state.LoadBalancers)

		if lbc < 0 {
			return fmt.Errorf("failed to find requested loadbalancer %s from which to move the api endpoint from", change.CurrentEndpointId)
		}

		if lbd < 0 {
			return fmt.Errorf("failed to find requested loadbalancer %s to which to move the api endpoint", change.DesiredEndpointId)
		}

		state.LoadBalancers[lbc].UsedApiEndpoint = false
		state.LoadBalancers[lbd].UsedApiEndpoint = true

		oldEndpoint = state.LoadBalancers[lbc].Dns.Endpoint
		newEndpoint = state.LoadBalancers[lbd].Dns.Endpoint
	}

	logger.Debug().Str("LB-cluster", state.K8S.ClusterInfo.Id()).Msgf("Changing the API endpoint from %s to %s", oldEndpoint, newEndpoint)
	return utils.ChangeAPIEndpoint(state.K8S.ClusterInfo.Name, oldEndpoint, newEndpoint, outputDirectory, processLimit)
}
