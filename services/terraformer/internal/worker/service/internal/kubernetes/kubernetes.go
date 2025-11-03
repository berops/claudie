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

	DesiredState *spec.K8SclusterV2
	CurrentState *spec.K8SclusterV2

	// Signals whether to export port 6443 on the
	// control plane nodes of the cluster.
	// This value is passed down when generating
	// the terraform templates.
	ExportPort6443 bool

	// SpawnProcessLimit limits the number of spawned tofu processes.
	SpawnProcessLimit *semaphore.Weighted
}

func (k *K8Scluster) Id() string {
	state := k.DesiredState
	if state == nil {
		state = k.CurrentState
	}
	return state.ClusterInfo.Id()
}

func (k *K8Scluster) HasCurrentState() bool { return k.CurrentState != nil }
func (k *K8Scluster) IsKubernetes() bool    { return true }

func (k *K8Scluster) Build(logger zerolog.Logger) error {
	logger.Info().Msgf("Building K8S Cluster %s", k.DesiredState.ClusterInfo.Name)

	var currentClusterInfo *spec.ClusterInfoV2

	// Check if current cluster was defined, to avoid access of unreferenced memory
	if k.CurrentState != nil {
		currentClusterInfo = k.CurrentState.ClusterInfo
	}

	cluster := cluster_builder.ClusterBuilder{
		DesiredClusterInfo: k.DesiredState.ClusterInfo,
		CurrentClusterInfo: currentClusterInfo,
		ProjectName:        k.ProjectName,
		ClusterType:        spec.ClusterType_K8s,
		K8sInfo: cluster_builder.K8sInfo{
			ExportPort6443: k.ExportPort6443,
		},
		SpawnProcessLimit: k.SpawnProcessLimit,
	}

	if err := cluster.CreateNodepools(); err != nil {
		return fmt.Errorf("%w: error while creating the K8s cluster %s : %w", ErrCreateNodePools, k.DesiredState.ClusterInfo.Name, err)
	}

	return nil
}

func (k *K8Scluster) Destroy(logger zerolog.Logger) error {
	logger.Info().Msgf("Destroying K8S Cluster %s", k.CurrentState.ClusterInfo.Name)
	cluster := cluster_builder.ClusterBuilder{
		CurrentClusterInfo: k.CurrentState.ClusterInfo,

		ProjectName:       k.ProjectName,
		ClusterType:       spec.ClusterType_K8s,
		SpawnProcessLimit: k.SpawnProcessLimit,
	}

	err := cluster.DestroyNodepools()
	if err != nil {
		return fmt.Errorf("error while destroying the K8s cluster %s : %w", k.CurrentState.ClusterInfo.Name, err)
	}

	return nil
}

func (k *K8Scluster) UpdateCurrentState() { k.CurrentState = k.DesiredState }
