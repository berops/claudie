package loadbalancer

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	cluster_builder "github.com/berops/claudie/services/terraformer/internal/worker/service/internal/cluster-builder"
	"github.com/rs/zerolog"
	"golang.org/x/sync/semaphore"
)

type DNS struct {
	ProjectName string
	Cluster     *spec.LBcluster
	// SpawnProcessLimit  limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
}

func (b *DNS) ReconcileRecords(_ context.Context, logger zerolog.Logger) error {
	builder := cluster_builder.DnsBuilder{
		ClusterName:       b.Cluster.ClusterInfo.Name,
		ClusterHash:       b.Cluster.ClusterInfo.Hash,
		ClusterId:         b.Cluster.ClusterInfo.Id(),
		InputManifest:     b.ProjectName,
		SpawnProcessLimit: b.SpawnProcessLimit,
	}

	if b.Cluster.Dns == nil {
		// No DNS nothing to reconcile.
		return nil
	}

	nodeIPs := nodepools.PublicEndpoints(b.Cluster.ClusterInfo.NodePools) // include all of the nodes.
	if err := builder.Init(logger, nodeIPs, b.Cluster.Dns); err != nil {
		return err
	}
	defer builder.Cleanup()

	if err := builder.ReconcileRecords(); err != nil {
		return fmt.Errorf("error while reconciling dns records for cluster %q: %w", builder.ClusterId, err)
	}
	return nil
}

func (b *DNS) DestroyRecords(_ context.Context, logger zerolog.Logger) error {
	builder := cluster_builder.DnsBuilder{
		ClusterName:       b.Cluster.ClusterInfo.Name,
		ClusterHash:       b.Cluster.ClusterInfo.Hash,
		ClusterId:         b.Cluster.ClusterInfo.Id(),
		InputManifest:     b.ProjectName,
		SpawnProcessLimit: b.SpawnProcessLimit,
	}

	if b.Cluster.Dns == nil {
		// No DNS nothing to reconcile.
		return nil
	}

	nodeIPs := nodepools.PublicEndpoints(b.Cluster.ClusterInfo.NodePools) // include all of the nodes.
	if err := builder.Init(logger, nodeIPs, b.Cluster.Dns); err != nil {
		return err
	}
	defer builder.Cleanup()

	if err := builder.DestroyRecords(); err != nil {
		return fmt.Errorf("error while destroying dns records for cluster %q: %w", builder.ClusterId, err)
	}
	return nil
}
