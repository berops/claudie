package service

import (
	"context"

	"github.com/berops/claudie/internal/concurrent"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/kubernetes"
	"github.com/berops/claudie/services/terraformer/internal/worker/service/internal/loadbalancer"
	"github.com/berops/claudie/services/terraformer/internal/worker/store"
	"github.com/rs/zerolog"

	"golang.org/x/sync/semaphore"
)

type Cluster interface {
	// Destroys all of the infrastructure of the cluster.
	DestroyAll(ctx context.Context, logger zerolog.Logger, s3 store.S3StateStorage) error

	// Id returns a cluster ID for the cluster.
	Id() string

	// whether the cluster is a kubernetes cluster, if not it is a loadbalancer.
	IsKubernetes() bool
}

func destroy(
	logger zerolog.Logger,
	stores Stores,
	projectName string,
	processLimit *semaphore.Weighted,
	tracker Tracker,
) {
	logger.Info().Msg("Destroying infrastructure")

	var clusters []Cluster

	action, ok := tracker.Task.Do.(*spec.Task_Delete)
	if !ok {
		logger.
			Warn().
			Msgf("received task with action %T while wanting to destroy infrastructure, assuming the task was misscheduled, ignoring", tracker.Task.Do)
		return
	}

	k8s, loadbalancers := action.Delete.K8S, action.Delete.LoadBalancers
	if k8s == nil {
		logger.
			Warn().
			Msg("delete task validation failed, required kubernetes state to be present, but is missing, ignoring")
		return
	}

	clusters = append(clusters, &kubernetes.K8Scluster{
		ProjectName:       projectName,
		Cluster:           k8s,
		SpawnProcessLimit: processLimit,
	})

	for _, lb := range loadbalancers {
		if lb == nil {
			logger.
				Warn().
				Msg("delete task validation failed, required loadbalancer state to be present, but is missing, ignoring")
			return
		}

		clusters = append(clusters, &loadbalancer.LBcluster{
			ProjectName:       projectName,
			Cluster:           lb,
			SpawnProcessLimit: processLimit,
		})
	}

	ids := make([]string, len(clusters))
	errs := make([]error, len(clusters))

	err := concurrent.Exec(clusters, func(idx int, cluster Cluster) error {
		buildLogger := logger.With().Str("cluster", cluster.Id()).Logger()
		ids[idx] = cluster.Id()
		errs[idx] = cluster.DestroyAll(context.Background(), buildLogger, stores.s3)
		return errs[idx]
	})
	if err != nil {
		logger.Err(err).Msg("Failed to destroy clusters")
		tracker.Diagnostics.Push(err)
		// Some of the provided clusters didn't destroy successfully.
		// Since we still want to report the partially destroyed infrastructure
		// back to the caller we fallthrough here.
		//
		// fallthrough
	}

	var (
		k8sId string
		lbIds []string
	)

	for i, c := range clusters {
		if errs[i] == nil {
			if c.IsKubernetes() {
				k8sId = ids[i]
			} else {
				lbIds = append(lbIds, ids[i])
			}
		}
	}

	// All of the successfully deleted
	// clusters can be cleared.
	infraClear := tracker.Result.Clear()
	if k8sId != "" {
		infraClear.Kubernetes()
	}
	infraClear.LoadBalancers(lbIds...)
	infraClear.Commit()
}
