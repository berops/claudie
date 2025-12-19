package kubernetes

import (
	"errors"
	"fmt"

	"github.com/berops/claudie/proto/pb/spec"
	cluster_builder "github.com/berops/claudie/services/terraformer/internal/worker/service/internal/cluster-builder"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

var (
	// ErrCreateNodePools is returned when an error occurs during the creation of the desired nodepools.
	ErrCreateNodePools = errors.New("failed to create desired nodepools")
)

type K8Scluster struct {
	ProjectName string
	Cluster     *spec.K8Scluster

	// Signals whether to export port 6443 on the
	// control plane nodes of the cluster.
	// This value is passed down when generating
	// the terraform templates.
	ExportPort6443 bool

	// SpawnProcessLimit limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
}

func (k *K8Scluster) Id() string         { return k.Cluster.ClusterInfo.Id() }
func (k *K8Scluster) IsKubernetes() bool { return true }

func (k *K8Scluster) Build(logger zerolog.Logger) error {
	logger.Info().Msgf("Building K8S Cluster %s", k.Cluster.ClusterInfo.Name)

	cluster := cluster_builder.ClusterBuilder{
		ClusterInfo: k.Cluster.ClusterInfo,
		ProjectName: k.ProjectName,
		ClusterType: cluster_builder.Kubernetes,
		K8sInfo: cluster_builder.K8sInfo{
			ExportPort6443: k.ExportPort6443,
		},
		SpawnProcessLimit: k.SpawnProcessLimit,
	}

	if err := cluster.CreateNodepools(); err != nil {
		return fmt.Errorf("%w: error while creating the K8s cluster %s : %w", ErrCreateNodePools, k.Cluster.ClusterInfo.Name, err)
	}

	return nil
}

func (k *K8Scluster) Destroy(logger zerolog.Logger) error {
	logger.Info().Msgf("Destroying K8S Cluster %s", k.Cluster.ClusterInfo.Name)
	cluster := cluster_builder.ClusterBuilder{
		ClusterInfo:       k.Cluster.ClusterInfo,
		ProjectName:       k.ProjectName,
		ClusterType:       cluster_builder.Kubernetes,
		SpawnProcessLimit: k.SpawnProcessLimit,
	}

	if err := cluster.DestroyNodepools(); err != nil {
		return fmt.Errorf("error while destroying the K8s cluster %s : %w", k.Cluster.ClusterInfo.Name, err)
	}

	return nil
}
