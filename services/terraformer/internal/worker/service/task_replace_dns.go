package service

import (
	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
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
	idx := clusters.IndexLoadbalancerById(action.Replace.Handle, action.State.LoadBalancers)
	if idx < 0 {
		logger.
			Warn().
			Msgf("Can't replace DNS for loadbalancer %q that is missing from the received state", action.Replace.Handle)
		return
	}

	lb := action.State.LoadBalancers[idx]
	if lb.Dns != nil {
		dns := loadbalancer.DNS{
			ProjectName:       projectName,
			ClusterName:       lb.ClusterInfo.Name,
			ClusterHash:       lb.ClusterInfo.Hash,
			NodeIPs:           nodepools.PublicEndpoints(lb.ClusterInfo.NodePools),
			Dns:               lb.Dns,
			SpawnProcessLimit: processLimit,
		}

		if err := dns.DestroyDNSRecords(logger); err != nil {
			logger.Err(err).Msg("Failed to destroy DNS records")
			tracker.Diagnostics.Push(err)
			return
		}

		lb.Dns = nil
	}

	if action.Replace.Dns == nil {
		update := tracker.Result.Update()
		update.Loadbalancers(lb)
		update.Commit()
		return
	}

	lb.Dns = action.Replace.Dns
	dns := loadbalancer.DNS{
		ProjectName:       projectName,
		ClusterName:       lb.ClusterInfo.Name,
		ClusterHash:       lb.ClusterInfo.Hash,
		NodeIPs:           nodepools.PublicEndpoints(lb.ClusterInfo.NodePools),
		Dns:               lb.Dns,
		SpawnProcessLimit: processLimit,
	}

	if err := dns.CreateDNSRecords(logger); err != nil {
		logger.Err(err).Msg("Failed to create new DNS records")
		tracker.Diagnostics.Push(err)
		return
	}

	update := tracker.Result.Update()
	update.Loadbalancers(lb)
	update.Commit()
}
