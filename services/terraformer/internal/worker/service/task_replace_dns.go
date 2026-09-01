package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	cluster_builder "github.com/berops/claudie/services/terraformer/internal/worker/service/internal/cluster-builder"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

type ReplaceDns struct {
	State   *spec.Update_State
	Replace *spec.Update_TerraformerReplaceDns
}

func replaceDns(
	logger zerolog.Logger,
	projectName string,
	processLimit *semaphore.Weighted,
	action ReplaceDns,
	tracker Tracker,
) {
	logger.Info().Msg("Replacing DNS")
	idx := clusters.IndexLoadbalancerById(action.Replace.Handle, action.State.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't replace DNS for loadbalancer %q that is missing from the received state", action.Replace.Handle)
		return
	}

	lb := action.State.LoadBalancers[idx]
	dns := cluster_builder.DnsBuilder{
		ClusterName:       lb.ClusterInfo.Name,
		ClusterHash:       lb.ClusterInfo.Hash,
		ClusterId:         lb.ClusterInfo.Id(),
		InputManifest:     projectName,
		SpawnProcessLimit: processLimit,
	}

	if lb.Dns != nil {
		current := lb.Dns

		// If there is a current state update it to nil
		// on either a success or a failure. When reporting
		// back to the manager service it should recognize
		// that the DNS reported nil, and make a proper diff
		// to either revert back or move to the new DNS again.
		lb.Dns = nil
		update := tracker.Result.Update()
		update.Loadbalancers(lb)
		update.Commit()

		ips := nodepools.PublicEndpoints(lb.ClusterInfo.NodePools)
		if err := dns.Init(logger, ips, current); err != nil {
			logger.Err(err).Msg("Failed to prepare resources for destroying DNS records")
			tracker.Diagnostics.Push(err)
			return
		}

		if err := dns.DestroyRecords(); err != nil {
			dns.Cleanup()
			logger.Err(err).Msg("Failed to destroy DNS records")
			tracker.Diagnostics.Push(err)
			return
		}
		dns.Cleanup()
	}

	if action.Replace.Dns == nil {
		return
	}

	lb.Dns = action.Replace.Dns
	ips := nodepools.PublicEndpoints(lb.ClusterInfo.NodePools)
	if err := dns.Init(logger, ips, lb.Dns); err != nil {
		logger.Err(err).Msg("Failed to prepare resources for reconciling DNS records")
		tracker.Diagnostics.Push(err)
		return
	}
	defer dns.Cleanup()

	if err := dns.ReconcileRecords(); err != nil {
		logger.Err(err).Msg("Failed to create new DNS records")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb)
	update.Commit()
}
