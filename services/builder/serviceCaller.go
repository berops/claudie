package main

import (
	"fmt"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	ansibler "github.com/berops/claudie/services/ansibler/client"
	kubeEleven "github.com/berops/claudie/services/kube-eleven/client"
	kuber "github.com/berops/claudie/services/kuber/client"
	terraformer "github.com/berops/claudie/services/terraformer/client"
	"github.com/rs/zerolog/log"
)

type BuilderContext struct {
	projectName string

	cluster        *pb.K8Scluster
	desiredCluster *pb.K8Scluster

	loadbalancers        []*pb.LBcluster
	desiredLoadbalancers []*pb.LBcluster

	deletedLoadBalancers []*pb.LBcluster

	Workflow *pb.Workflow
}

func (ctx *BuilderContext) GetClusterName() string {
	if ctx.desiredCluster != nil {
		return ctx.desiredCluster.ClusterInfo.Name
	}
	if ctx.cluster != nil {
		return ctx.cluster.ClusterInfo.Name
	}

	// try to get the cluster name from the lbs if present
	if len(ctx.desiredLoadbalancers) != 0 {
		return ctx.desiredLoadbalancers[0].TargetedK8S
	}

	if len(ctx.loadbalancers) != 0 {
		return ctx.loadbalancers[0].TargetedK8S
	}

	if len(ctx.deletedLoadBalancers) != 0 {
		return ctx.deletedLoadBalancers[0].TargetedK8S
	}

	return ""
}

func buildCluster(ctx *BuilderContext, c pb.ContextBoxServiceClient) (*BuilderContext, error) {
	if err := callTerraformer(ctx, c); err != nil {
		return nil, fmt.Errorf("error in Terraformer for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	if err := callAnsibler(ctx, c); err != nil {
		return nil, fmt.Errorf("error in Ansibler for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	if err := callKubeEleven(ctx, c); err != nil {
		return nil, fmt.Errorf("error in Kube-eleven for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	if err := callKuber(ctx, c); err != nil {
		return nil, fmt.Errorf("error in Kuber for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	return ctx, nil
}

// callTerraformer passes config to terraformer for building the infra
func callTerraformer(ctx *BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	description := ctx.Workflow.Description

	ctx.Workflow.Stage = pb.Workflow_TERRAFORMER
	ctx.Workflow.Description = fmt.Sprintf("%s building infrastructure", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	cc, err := utils.GrpcDialWithInsecure("terraformer", envs.TerraformerURL)
	if err != nil {
		return err
	}
	defer utils.CloseClientConnection(cc)

	c := pb.NewTerraformerServiceClient(cc)
	log.Info().Msgf("Calling BuildInfrastructure on Terraformer for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)

	req := &pb.BuildInfrastructureRequest{
		Current:     ctx.cluster,
		Desired:     ctx.desiredCluster,
		CurrentLbs:  ctx.loadbalancers,
		DesiredLbs:  ctx.desiredLoadbalancers,
		ProjectName: ctx.projectName,
	}

	res, err := terraformer.BuildInfrastructure(c, req)
	if err != nil {
		return err
	}

	ctx.cluster = res.Current
	ctx.desiredCluster = res.Desired
	ctx.loadbalancers = res.CurrentLbs
	ctx.desiredLoadbalancers = res.DesiredLbs

	ctx.Workflow.Description = description
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}
	log.Info().Msgf("BuildInfrastructure on Terraformer for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)
	return nil
}

// callAnsibler passes config to ansibler to set up VPN
func callAnsibler(ctx *BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	description := ctx.Workflow.Description

	ctx.Workflow.Stage = pb.Workflow_ANSIBLER
	ctx.Workflow.Description = fmt.Sprintf("%s tearing down loadbalancers", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	cc, err := utils.GrpcDialWithInsecure("ansibler", envs.AnsiblerURL)
	if err != nil {
		return err
	}
	defer utils.CloseClientConnection(cc)

	c := pb.NewAnsiblerServiceClient(cc)

	// Call TearDownLoadbalancers only when its needed.
	apiEndpoint := ""
	if len(ctx.deletedLoadBalancers) > 0 {
		log.Info().Msgf("Calling TearDownLoadbalancers on Ansibler for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
		teardownRes, err := ansibler.TeardownLoadBalancers(c, &pb.TeardownLBRequest{
			Desired:     ctx.desiredCluster,
			DesiredLbs:  ctx.desiredLoadbalancers,
			DeletedLbs:  ctx.deletedLoadBalancers,
			ProjectName: ctx.projectName,
		})
		if err != nil {
			return err
		}
		log.Info().Msgf("TearDownLoadbalancers on Ansibler for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

		ctx.desiredCluster = teardownRes.Desired
		ctx.desiredLoadbalancers = teardownRes.DesiredLbs
		ctx.deletedLoadBalancers = teardownRes.DeletedLbs
		apiEndpoint = teardownRes.PreviousAPIEndpoint
	}

	ctx.desiredCluster = teardownRes.Desired
	ctx.desiredLoadbalancers = teardownRes.DesiredLbs
	ctx.deletedLoadBalancers = teardownRes.DeletedLbs

	ctx.Workflow.Description = fmt.Sprintf("%s installing VPN", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	log.Info().Msgf("Calling InstallVPN on Ansibler for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	installRes, err := ansibler.InstallVPN(c, &pb.InstallRequest{
		Desired:     ctx.desiredCluster,
		DesiredLbs:  ctx.desiredLoadbalancers,
		ProjectName: ctx.projectName,
	})
	if err != nil {
		return err
	}
	log.Info().Msgf("InstallVPN on Ansibler for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

	ctx.desiredCluster = installRes.Desired
	ctx.desiredLoadbalancers = installRes.DesiredLbs

	ctx.Workflow.Description = fmt.Sprintf("%s installing node requirements", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	log.Info().Msgf("Calling InstallNodeRequirements on Ansibler for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	installRes, err = ansibler.InstallNodeRequirements(c, &pb.InstallRequest{
		Desired:     ctx.desiredCluster,
		DesiredLbs:  ctx.desiredLoadbalancers,
		ProjectName: ctx.projectName,
	})
	if err != nil {
		return err
	}
	log.Info().Msgf("InstallNodeRequirements on Ansibler for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

	ctx.desiredCluster = installRes.Desired
	ctx.desiredLoadbalancers = installRes.DesiredLbs

	ctx.Workflow.Description = fmt.Sprintf("%s setting up loadbalancers", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	log.Info().Msgf("Calling SetUpLoadbalancers on Ansibler for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	setUpRes, err := ansibler.SetUpLoadbalancers(c, &pb.SetUpLBRequest{
		Desired:             ctx.desiredCluster,
		CurrentLbs:          ctx.loadbalancers,
		DesiredLbs:          ctx.desiredLoadbalancers,
		PreviousAPIEndpoint: apiEndpoint,
		ProjectName:         ctx.projectName,
	})
	if err != nil {
		return err
	}
	log.Info().Msgf("SetUpLoadbalancers on Ansibler for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

	ctx.desiredCluster = setUpRes.Desired
	ctx.loadbalancers = setUpRes.CurrentLbs
	ctx.desiredLoadbalancers = setUpRes.DesiredLbs

	ctx.Workflow.Description = description
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	return nil
}

// callKubeEleven passes config to kubeEleven to bootstrap k8s cluster
func callKubeEleven(ctx *BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	description := ctx.Workflow.Description

	ctx.Workflow.Stage = pb.Workflow_KUBE_ELEVEN
	ctx.Workflow.Description = fmt.Sprintf("%s building kubernetes cluster", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	cc, err := utils.GrpcDialWithInsecure("kube-eleven", envs.KubeElevenURL)
	if err != nil {
		return err
	}
	defer utils.CloseClientConnection(cc)

	c := pb.NewKubeElevenServiceClient(cc)

	log.Info().Msgf("Calling BuildCluster on Kube-eleven for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)

	res, err := kubeEleven.BuildCluster(c, &pb.BuildClusterRequest{
		Desired:     ctx.desiredCluster,
		DesiredLbs:  ctx.desiredLoadbalancers,
		ProjectName: ctx.projectName,
	})

	if err != nil {
		return err
	}
	log.Info().Msgf("BuildCluster on Kube-eleven for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

	ctx.desiredCluster = res.Desired
	ctx.desiredLoadbalancers = res.DesiredLbs

	ctx.Workflow.Description = description
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}
	return nil
}

// callKuber passes config to Kuber to apply any additional resources via kubectl
func callKuber(ctx *BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	description := ctx.Workflow.Description

	ctx.Workflow.Stage = pb.Workflow_KUBER
	ctx.Workflow.Description = fmt.Sprintf("%s setting up storage", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	cc, err := utils.GrpcDialWithInsecure("kuber", envs.KuberURL)
	if err != nil {
		return err
	}
	defer utils.CloseClientConnection(cc)

	c := pb.NewKuberServiceClient(cc)

	// If previous cluster had loadbalancers, and the new one does not, the old scrape config will be removed.
	if len(ctx.desiredLoadbalancers) == 0 && len(ctx.loadbalancers) > 0 {
		log.Info().Msgf("Calling RemoveScrapeConfig on Kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
		if _, err := kuber.RemoveLbScrapeConfig(c, &pb.RemoveLbScrapeConfigRequest{
			Cluster: ctx.desiredCluster,
		}); err != nil {
			return err
		}
		log.Info().Msgf("RemoveScrapeConfig on Kuber for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)
	}

	// Create a scrape-config if there are loadbalancers in the new/updated cluster
	if len(ctx.desiredLoadbalancers) > 0 {
		log.Info().Msgf("Calling StoreLbScrapeConfig on Kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
		if _, err := kuber.StoreLbScrapeConfig(c, &pb.StoreLbScrapeConfigRequest{
			Cluster:              ctx.desiredCluster,
			DesiredLoadbalancers: ctx.desiredLoadbalancers,
		}); err != nil {
			return err
		}
		log.Info().Msgf("StoreLbScrapeConfig on Kuber for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)
	}

	log.Info().Msgf("Calling SetUpStorage on Kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	resStorage, err := kuber.SetUpStorage(c, &pb.SetUpStorageRequest{DesiredCluster: ctx.desiredCluster})
	if err != nil {
		return err
	}
	log.Info().Msgf("SetUpStorage on Kuber for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

	ctx.desiredCluster = resStorage.DesiredCluster

	ctx.Workflow.Description = fmt.Sprintf("%s creating kubeconfig as secret", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	log.Info().Msgf("Calling StoreKubeconfig on kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	if _, err := kuber.StoreKubeconfig(c, &pb.StoreKubeconfigRequest{Cluster: ctx.desiredCluster}); err != nil {
		return err
	}
	log.Info().Msgf("StoreKubeconfig on Kuber for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

	ctx.Workflow.Description = fmt.Sprintf("%s creating cluster metadata as secret", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	log.Info().Msgf("Calling StoreNodeMetadata on kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	if _, err := kuber.StoreClusterMetadata(c, &pb.StoreClusterMetadataRequest{Cluster: ctx.desiredCluster}); err != nil {
		return err
	}
	log.Info().Msgf("StoreNodeMetadata on Kuber for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

	log.Info().Msgf("Calling PatchNodes on kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	if _, err := kuber.PatchNodes(c, &pb.PatchNodeTemplateRequest{Cluster: ctx.desiredCluster}); err != nil {
		return err
	}

	if utils.IsAutoscaled(ctx.desiredCluster) {
		// Set up Autoscaler if desired state is autoscaled
		log.Info().Msgf("Calling SetUpClusterAutoscaler on kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
		if _, err := kuber.SetUpClusterAutoscaler(c, &pb.SetUpClusterAutoscalerRequest{ProjectName: ctx.projectName, Cluster: ctx.desiredCluster}); err != nil {
			return err
		}
	} else if utils.IsAutoscaled(ctx.cluster) {
		// Destroy Autoscaler if current state is autoscaled, but desired is not
		log.Info().Msgf("Calling DestroyClusterAutoscaler on kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
		if _, err := kuber.DestroyClusterAutoscaler(c, &pb.DestroyClusterAutoscalerRequest{ProjectName: ctx.projectName, Cluster: ctx.cluster}); err != nil {
			return err
		}
	}

	ctx.Workflow.Description = description
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	return nil
}

// destroyCluster destroys existing clusters infra for a config by calling Terraformer and Kuber
func destroyCluster(ctx *BuilderContext, c pb.ContextBoxServiceClient) error {
	// Destroy infra
	if err := destroyConfigTerraformer(ctx, c); err != nil {
		return fmt.Errorf("error in destroy config Terraformer for config %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	cc, err := utils.GrpcDialWithInsecure("kuber", envs.KuberURL)
	if err != nil {
		return err
	}
	defer utils.CloseClientConnection(cc)
	c := pb.NewKuberServiceClient(cc)

	// Delete cluster metadata
	if err := deleteClusterData(ctx, c); err != nil {
		return fmt.Errorf("error in delete kubeconfig for config %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	// Destroy Autoscaler if current state is autoscaled
	if utils.IsAutoscaled(ctx.cluster) {
		log.Info().Msgf("Calling DestroyClusterAutoscaler on kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
		if _, err := kuber.DestroyClusterAutoscaler(c, &pb.DestroyClusterAutoscalerRequest{ProjectName: ctx.projectName, Cluster: ctx.cluster}); err != nil {
			return err
		}
	}

	return nil
}

// destroyConfigTerraformer calls terraformer's DestroyInfrastructure function
func destroyConfigTerraformer(ctx *BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	description := ctx.Workflow.Description

	ctx.Workflow.Stage = pb.Workflow_DESTROY_TERRAFORMER
	ctx.Workflow.Description = fmt.Sprintf("%s destroying infrastructure", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	cc, err := utils.GrpcDialWithInsecure("terraformer", envs.TerraformerURL)
	if err != nil {
		return err
	}
	defer utils.CloseClientConnection(cc)

	log.Info().Msgf("Calling DestroyInfrastructure on Terraformer for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	c := pb.NewTerraformerServiceClient(cc)

	if _, err = terraformer.DestroyInfrastructure(c, &pb.DestroyInfrastructureRequest{
		ProjectName: ctx.projectName,
		Current:     ctx.cluster,
		CurrentLbs:  ctx.loadbalancers,
	}); err != nil {
		return fmt.Errorf("error while destroying infrastructure  cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}
	ctx.Workflow.Description = description
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}
	log.Info().Msgf("DestroyInfrastructure on Terraformer for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)
	return nil
}

// deleteClusterData deletes the kubeconfig and cluster metadata.
func deleteClusterData(ctx *BuilderContext, cboxClient pb.ContextBoxServiceClient) error {
	if ctx.cluster == nil {
		return nil
	}
	description := ctx.Workflow.Description

	ctx.Workflow.Stage = pb.Workflow_DESTROY_KUBER
	ctx.Workflow.Description = fmt.Sprintf("%s deleting kubeconfig secret", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	log.Info().Msgf("Calling DeleteKubeconfig on Kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	if _, err := kuber.DeleteKubeconfig(c, &pb.DeleteKubeconfigRequest{Cluster: ctx.cluster}); err != nil {
		return fmt.Errorf("error while deleting kubeconfig for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	ctx.Workflow.Description = fmt.Sprintf("%s deleting cluster metadata secret", description)
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}

	log.Info().Msgf("Calling DeleteClusterMetadata on kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	if _, err := kuber.DeleteClusterMetadata(c, &pb.DeleteClusterMetadataRequest{Cluster: ctx.cluster}); err != nil {
		return fmt.Errorf("error while deleting metadata for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}
	log.Info().Msgf("DeleteKubeconfig on Kuber for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)
	ctx.Workflow.Description = description
	if err := updateWorkflowStateInDB(ctx.projectName, ctx.GetClusterName(), ctx.Workflow, cboxClient); err != nil {
		return err
	}
	return nil
}

// callDeleteNodes calls Kuber.DeleteNodes which will safely delete nodes from cluster
func callDeleteNodes(master, worker []string, cluster *pb.K8Scluster) (*pb.K8Scluster, error) {
	cc, err := utils.GrpcDialWithInsecure("kuber", envs.KuberURL)
	if err != nil {
		return nil, err
	}
	defer utils.CloseClientConnection(cc)

	// Creating the client
	c := pb.NewKuberServiceClient(cc)
	log.Info().Msgf("Calling DeleteNodes on Kuber for cluster %s", cluster.ClusterInfo.Name)
	resDelete, err := kuber.DeleteNodes(c, &pb.DeleteNodesRequest{MasterNodes: master, WorkerNodes: worker, Cluster: cluster})
	if err != nil {
		return nil, err
	}
	log.Info().Msgf("DeleteNodes on Kuber for cluster %s finished successfully", cluster.ClusterInfo.Name)
	return resDelete.Cluster, nil
}
