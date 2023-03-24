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
}

func (ctx *BuilderContext) GetClusterName() string {
	if ctx.desiredCluster != nil {
		return ctx.desiredCluster.ClusterInfo.Name
	}
	if ctx.cluster != nil {
		return ctx.cluster.ClusterInfo.Name
	}

	// try to get the cluster name from the lbs if present
	if len(ctx.loadbalancers) != 0 {
		return ctx.loadbalancers[0].TargetedK8S
	}

	if len(ctx.desiredLoadbalancers) != 0 {
		return ctx.desiredLoadbalancers[0].TargetedK8S
	}

	if len(ctx.deletedLoadBalancers) != 0 {
		return ctx.deletedLoadBalancers[0].TargetedK8S
	}

	return ""
}

func buildCluster(ctx *BuilderContext) (*BuilderContext, error) {
	if err := callTerraformer(ctx); err != nil {
		return nil, fmt.Errorf("error in Terraformer for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	if err := callAnsibler(ctx); err != nil {
		return nil, fmt.Errorf("error in Ansibler for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	if err := callKubeEleven(ctx); err != nil {
		return nil, fmt.Errorf("error in Kube-eleven for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	if err := callKuber(ctx); err != nil {
		return nil, fmt.Errorf("error in Kuber for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	return ctx, nil
}

// callTerraformer passes config to terraformer for building the infra
func callTerraformer(ctx *BuilderContext) error {
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
	log.Info().Msgf("BuildInfrastructure on Terraformer for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)
	return nil
}

// callAnsibler passes config to ansibler to set up VPN
func callAnsibler(ctx *BuilderContext) error {
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

	return nil
}

// callKubeEleven passes config to kubeEleven to bootstrap k8s cluster
func callKubeEleven(ctx *BuilderContext) error {
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

	return nil
}

// callKuber passes config to Kuber to apply any additional resources via kubectl
func callKuber(ctx *BuilderContext) error {
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

	log.Info().Msgf("Calling StoreKubeconfig on Kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	if _, err := kuber.StoreKubeconfig(c, &pb.StoreKubeconfigRequest{Cluster: ctx.desiredCluster}); err != nil {
		return err
	}
	log.Info().Msgf("StoreKubeconfig on Kuber for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

	log.Info().Msgf("Calling StoreNodeMetadata on Kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	if _, err := kuber.StoreClusterMetadata(c, &pb.StoreClusterMetadataRequest{Cluster: ctx.desiredCluster}); err != nil {
		return err
	}
	log.Info().Msgf("StoreNodeMetadata on Kuber for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

	return nil
}

// destroyConfig destroys existing clusters infra for a config by calling Terraformer and Kuber
func destroyCluster(ctx *BuilderContext) error {
	if err := destroyConfigTerraformer(ctx); err != nil {
		return fmt.Errorf("error in destroy config Terraformer for config %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	if err := deleteClusterData(ctx); err != nil {
		return fmt.Errorf("error in delete kubeconfig for config %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	return nil
}

// destroyConfigTerraformer calls terraformer's DestroyInfrastructure function
func destroyConfigTerraformer(ctx *BuilderContext) error {
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
	log.Info().Msgf("DestroyInfrastructure on Terraformer for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)

	return nil
}

// deleteClusterData deletes the kubeconfig and cluster metadata.
func deleteClusterData(ctx *BuilderContext) error {
	if ctx.cluster == nil {
		return nil
	}

	cc, err := utils.GrpcDialWithInsecure("kuber", envs.KuberURL)
	if err != nil {
		return err
	}
	defer utils.CloseClientConnection(cc)

	c := pb.NewKuberServiceClient(cc)

	log.Info().Msgf("Calling DeleteKubeconfig on Kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	if _, err := kuber.DeleteKubeconfig(c, &pb.DeleteKubeconfigRequest{Cluster: ctx.cluster}); err != nil {
		return fmt.Errorf("error while deleting kubeconfig for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}

	log.Info().Msgf("Calling DeleteClusterMetadata on kuber for cluster %s project %s", ctx.GetClusterName(), ctx.projectName)
	if _, err = kuber.DeleteClusterMetadata(c, &pb.DeleteClusterMetadataRequest{Cluster: ctx.cluster}); err != nil {
		return fmt.Errorf("error while deleting metadata for cluster %s project %s : %w", ctx.GetClusterName(), ctx.projectName, err)
	}
	log.Info().Msgf("DeleteKubeconfig on Kuber for cluster %s project %s finished successfully", ctx.GetClusterName(), ctx.projectName)
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
