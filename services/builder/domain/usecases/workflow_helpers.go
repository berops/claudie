package usecases

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/envs"
	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	kuber "github.com/berops/claudie/services/kuber/client"
)

func (u *Usecases) buildCluster(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) (*utils.BuilderContext, error) {
	if err := u.reconcileInfrastructure(ctx, cboxClient); err != nil {
		return ctx, fmt.Errorf("error in Terraformer for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	if err := u.callAnsibler(ctx, cboxClient); err != nil {
		return ctx, fmt.Errorf("error in Ansibler for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	if err := u.buildK8sCluster(ctx, cboxClient); err != nil {
		return ctx, fmt.Errorf("error in Kube-eleven for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	if err := u.reconcileK8s(ctx, cboxClient); err != nil {
		return ctx, fmt.Errorf("error in Kuber for cluster %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	return ctx, nil
}

// destroyCluster destroys existing clusters infra for a config by calling Terraformer and Kuber
func (u *Usecases) destroyCluster(ctx *utils.BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	// Destroy infra
	if err := u.destroyInfrastructure(ctx, cboxClient); err != nil {
		return fmt.Errorf("error in destroy config Terraformer for config %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	cc, err := cutils.GrpcDialWithRetryAndBackoff("kuber", envs.KuberURL)
	if err != nil {
		return err
	}
	defer cutils.CloseClientConnection(cc)
	kc := pb.NewKuberServiceClient(cc)

	// Delete cluster metadata
	if err := u.deleteClusterData(ctx, cboxClient); err != nil {
		return fmt.Errorf("error in delete kubeconfig for config %s project %s : %w", ctx.GetClusterName(), ctx.ProjectName, err)
	}

	// Destroy Autoscaler if current state is autoscaled
	if cutils.IsAutoscaled(ctx.CurrentCluster) {
		log.Info().Str("project", ctx.ProjectName).Str("cluster", ctx.GetClusterName()).Msg("Calling DestroyClusterAutoscaler on kuber")
		if err := kuber.DestroyClusterAutoscaler(kc, &pb.DestroyClusterAutoscalerRequest{ProjectName: ctx.ProjectName, Cluster: ctx.CurrentCluster}); err != nil {
			return err
		}
	}

	return nil
}

// destroyConfig destroys all the current state of the config.
func (u *Usecases) destroyConfig(config *pb.Config, clusterView *cutils.ClusterView, c pb.ContextBoxServiceClient) error {
	if err := cutils.ConcurrentExec(config.CurrentState.Clusters, func(_ int, cluster *pb.K8Scluster) error {
		err := u.destroyCluster(&utils.BuilderContext{
			ProjectName:          config.Name,
			CurrentCluster:       cluster,
			CurrentLoadbalancers: clusterView.Loadbalancers[cluster.ClusterInfo.Name],
			Workflow:             clusterView.ClusterWorkflows[cluster.ClusterInfo.Name],
		}, c)

		if err != nil {
			clusterView.SetWorkflowError(cluster.ClusterInfo.Name, err)
			return err
		}

		clusterView.SetWorkflowDone(cluster.ClusterInfo.Name)
		return nil
	}); err != nil {
		return err
	}

	return u.ContextBox.DeleteConfig(config, c)
}
