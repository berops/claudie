package usecases

import (
	"context"
	"fmt"

	"github.com/berops/claudie/internal/clusters"
	builder "github.com/berops/claudie/services/builder/internal"
	"github.com/rs/zerolog"
)

// reconcileK8sCluster reconciles desired k8s cluster via kube-eleven.
func (u *Usecases) reconcileK8sCluster(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	var lbApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpoint(work.DesiredLoadbalancers); ep != nil {
		lbApiEndpoint = ep.Dns.Endpoint
	}

	res, err := u.KubeEleven.BuildCluster(work, lbApiEndpoint, u.KubeEleven.GetClient())
	if err != nil {
		return fmt.Errorf("error while building kubernetes cluster %s project %s : %w", work.Id(), work.ProjectName, err)
	}

	work.DesiredCluster = res.Desired
	return nil
}

func (u *Usecases) destroyK8sCluster(_ context.Context, work *builder.Context, _ *zerolog.Logger) error {
	var lbApiEndpoint string
	if ep := clusters.FindAssignedLbApiEndpoint(work.CurrentLoadbalancers); ep != nil && ep.Dns != nil {
		lbApiEndpoint = ep.Dns.Endpoint
	}

	res, err := u.KubeEleven.DestroyCluster(work, lbApiEndpoint, u.KubeEleven.GetClient())
	if err != nil {
		return fmt.Errorf("error while destroying kubernetes cluster %s project %s: %w", work.Id(), work.ProjectName, err)
	}

	work.CurrentCluster = res.Current
	return nil
}
