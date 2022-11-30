package main

import (
	"fmt"
	"strings"

	"github.com/Berops/claudie/internal/envs"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	ansibler "github.com/Berops/claudie/services/ansibler/client"
	cbox "github.com/Berops/claudie/services/context-box/client"
	kubeEleven "github.com/Berops/claudie/services/kube-eleven/client"
	kuber "github.com/Berops/claudie/services/kuber/client"
	terraformer "github.com/Berops/claudie/services/terraformer/client"
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/errgroup"
)

// buildConfig is function used to build infra based on the desired state concurrently
func buildConfig(config *pb.Config, c pb.ContextBoxServiceClient, isTmpConfig bool, oldAPIEndpoints map[string]string) (err error) {
	log.Debug().Msgf("processConfig received config: %s, is tmpConfig: %t", config.GetName(), isTmpConfig)
	// call Terraformer to build infra
	currentState, desiredState, err := callTerraformer(config.GetCurrentState(), config.GetDesiredState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in Terraformer for config %s : %v; unable to save error message config: %w", config.Name, err, err1)
		}
		return fmt.Errorf("error in Terraformer for config %s : %w", config.Name, err)
	}
	config.CurrentState = currentState
	config.DesiredState = desiredState
	// call Ansibler to build VPN
	desiredState, err = callAnsibler(config.GetDesiredState(), config.GetCurrentState(), oldAPIEndpoints)
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in Ansibler for config %s : %v; unable to save error message config: %w", config.Name, err, err1)
		}
		return fmt.Errorf("error in Ansibler for config %s : %w", config.Name, err)
	}
	config.DesiredState = desiredState
	// call Kube-eleven to create K8s clusters
	desiredState, err = callKubeEleven(config.GetDesiredState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in KubeEleven for config %s : %v; unable to save error message config: %w", config.Name, err, err1)
		}
		return fmt.Errorf("error in KubeEleven for config %s : %w", config.Name, err)
	}
	config.DesiredState = desiredState

	// call Kuber to set up longhorn
	desiredState, err = callKuber(config.GetDesiredState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in Kuber for config %s : %v; unable to save error message config: %w", config.Name, err, err1)
		}
		return fmt.Errorf("error in Kuber for config %s : %w", config.Name, err)
	}
	config.DesiredState = desiredState

	if !isTmpConfig {
		log.Debug().Msgf("Saving the config %s", config.GetName())
		config.CurrentState = config.DesiredState // Update currentState
		err := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
		if err != nil {
			return fmt.Errorf("error while saving the config %s: %w", config.GetName(), err)
		}
	}

	return nil
}

// teardownLoadBalancers destroy the Load-Balancers (if any) for the config generated
// by the getDeletedClusterConfig function.
func teardownLoadBalancers(deleted, current, desired *pb.Project) (map[string]string, error) {
	conn, err := utils.GrpcDialWithInsecure("ansibler", envs.AnsiblerURL)
	if err != nil {
		return nil, err
	}

	log.Info().Msgf("Calling TeardownLoadBalancers on ansibler")

	resp, err := ansibler.TeardownLoadBalancers(pb.NewAnsiblerServiceClient(conn), &pb.TeardownLBRequest{
		CurrentState: current,
		DeletedState: deleted,
		DesiredState: desired,
	})

	if err != nil {
		return nil, err
	}

	return resp.OldApiEndpoinds, conn.Close()
}

// destroyConfig destroys existing clusters infra for a config, including the deletion
// of the config from the database, by calling Terraformer and Kuber and ContextBox services.
func destroyConfigAndDeleteDoc(config *pb.Config, c pb.ContextBoxServiceClient) error {
	err := destroyConfig(config, c)
	if err != nil {
		return fmt.Errorf("error while destroying config %s : %w", config.Name, err)
	}

	return cbox.DeleteConfigFromDB(c, config.Id, pb.IdType_HASH)
}

// destroyConfig destroys existing clusters infra for a config by calling Terraformer
// and Kuber
func destroyConfig(config *pb.Config, c pb.ContextBoxServiceClient) error {
	if err := destroyConfigTerraformer(config); err != nil {
		if err := saveErrorMessage(config, c, err); err != nil {
			return fmt.Errorf("failed to save error message: %w", err)
		}
		return fmt.Errorf("error in destroy config terraformer for config %s : %w", config.Name, err)
	}

	if err := deleteClusterData(config); err != nil {
		if err := saveErrorMessage(config, c, err); err != nil {
			return fmt.Errorf("failed to save error message for config %s : %w", config.Name, err)
		}
		return fmt.Errorf("error in delete kubeconfig for config %s : %w", config.Name, err)
	}
	return nil
}

// callTerraformer passes config to terraformer for building the infra
func callTerraformer(currentState *pb.Project, desiredState *pb.Project) (*pb.Project, *pb.Project, error) {
	// Create connection to Terraformer
	cc, err := utils.GrpcDialWithInsecure("terraformer", envs.TerraformerURL)
	if err != nil {
		return nil, nil, err
	}
	defer utils.CloseClientConnection(cc)
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	log.Info().Msgf("Calling BuildInfrastructure on terraformer for project %s", desiredState.Name)
	res, err := terraformer.BuildInfrastructure(c, &pb.BuildInfrastructureRequest{
		CurrentState: currentState,
		DesiredState: desiredState,
	})
	if err != nil {
		return nil, nil, err
	}

	return res.GetCurrentState(), res.GetDesiredState(), nil
}

// callAnsibler passes config to ansibler to set up VPN
func callAnsibler(desiredState, currentState *pb.Project, oldAPIEndpoints map[string]string) (*pb.Project, error) {
	cc, err := utils.GrpcDialWithInsecure("ansibler", envs.AnsiblerURL)
	if err != nil {
		return nil, err
	}
	defer utils.CloseClientConnection(cc)

	// Creating the client
	c := pb.NewAnsiblerServiceClient(cc)
	log.Info().Msgf("Calling InstallVPN on ansibler for project %s", desiredState.Name)
	installRes, err := ansibler.InstallVPN(c, &pb.InstallRequest{DesiredState: desiredState})
	if err != nil {
		return nil, err
	}
	log.Info().Msgf("Calling InstallNodeRequirements on ansibler for project %s", desiredState.Name)
	installRes, err = ansibler.InstallNodeRequirements(c, &pb.InstallRequest{DesiredState: installRes.DesiredState})
	if err != nil {
		return nil, err
	}
	log.Info().Msgf("Calling SetUpLoadbalancers on ansibler for project %s", desiredState.Name)
	setUpRes, err := ansibler.SetUpLoadbalancers(c, &pb.SetUpLBRequest{DesiredState: installRes.DesiredState, CurrentState: currentState, OldApiEndpoints: oldAPIEndpoints})
	if err != nil {
		return nil, err
	}
	return setUpRes.GetDesiredState(), nil
}

// callKubeEleven passes config to kubeEleven to bootstrap k8s cluster
func callKubeEleven(desiredState *pb.Project) (*pb.Project, error) {
	cc, err := utils.GrpcDialWithInsecure("kubeEleven", envs.KubeElevenURL)
	if err != nil {
		return nil, err
	}
	defer utils.CloseClientConnection(cc)
	// Creating the client
	c := pb.NewKubeElevenServiceClient(cc)
	log.Info().Msgf("Calling BuildCluster on kube-eleven for project %s", desiredState.Name)
	res, err := kubeEleven.BuildCluster(c, &pb.BuildClusterRequest{DesiredState: desiredState})
	if err != nil {
		return nil, err
	}

	return res.GetDesiredState(), nil
}

// callKuber passes config to Kuber to apply any additional resources via kubectl
func callKuber(desiredState *pb.Project) (*pb.Project, error) {
	cc, err := utils.GrpcDialWithInsecure("kuber", envs.KuberURL)
	if err != nil {
		return nil, err
	}
	defer utils.CloseClientConnection(cc)
	// Creating the client
	c := pb.NewKuberServiceClient(cc)
	log.Info().Msgf("Calling SetUpStorage on kuber for project %s", desiredState.Name)
	resStorage, err := kuber.SetUpStorage(c, &pb.SetUpStorageRequest{DesiredState: desiredState})
	if err != nil {
		return nil, err
	}

	var group errgroup.Group
	for _, cluster := range desiredState.Clusters {
		group.Go(func() error {
			log.Info().Msgf("Calling StoreKubeconfig on kuber for cluster %s", cluster.ClusterInfo.Name)
			if _, err := kuber.StoreKubeconfig(c, &pb.StoreKubeconfigRequest{Cluster: cluster}); err != nil {
				return err
			}

			log.Info().Msgf("Calling StoreNodeMetadata on kuber for cluster %s", cluster.ClusterInfo.Name)
			_, err := kuber.StoreClusterMetadata(c, &pb.StoreClusterMetadataRequest{Cluster: cluster})
			return err
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	return resStorage.GetDesiredState(), nil
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
	log.Info().Msgf("Calling DeleteNodes on kuber for cluster %s", cluster.ClusterInfo.Name)
	resDelete, err := kuber.DeleteNodes(c, &pb.DeleteNodesRequest{MasterNodes: master, WorkerNodes: worker, Cluster: cluster})
	if err != nil {
		return nil, err
	}
	return resDelete.Cluster, nil
}

// destroyConfigTerraformer calls terraformer's DestroyInfrastructure function
func destroyConfigTerraformer(config *pb.Config) error {
	// Trim "tcp://" substring from envs.TerraformerURL
	trimmedTerraformerURL := strings.ReplaceAll(envs.TerraformerURL, ":tcp://", "")

	cc, err := utils.GrpcDialWithInsecure("terraformer", trimmedTerraformerURL)
	if err != nil {
		return err
	}
	defer utils.CloseClientConnection(cc)

	log.Info().Msgf("Calling DestroyInfrastructure on terraformer for project %s", config.Name)
	c := pb.NewTerraformerServiceClient(cc)
	_, err = terraformer.DestroyInfrastructure(c, &pb.DestroyInfrastructureRequest{Config: config})
	return err
}

// deleteClusterData deletes the kubeconfig and cluster metadata.
func deleteClusterData(config *pb.Config) error {
	trimmedKuberURL := strings.ReplaceAll(envs.KuberURL, ":tcp://", "")

	cc, err := utils.GrpcDialWithInsecure("kuber", trimmedKuberURL)
	if err != nil {
		return err
	}
	defer utils.CloseClientConnection(cc)

	c := pb.NewKuberServiceClient(cc)

	var group errgroup.Group
	for _, cluster := range config.CurrentState.Clusters {
		group.Go(func() error {
			log.Info().Msgf("Calling DeleteKubeconfig on kuber for cluster %s", cluster.ClusterInfo.Name)
			if _, err := kuber.DeleteKubeconfig(c, &pb.DeleteKubeconfigRequest{Cluster: cluster}); err != nil {
				return err
			}

			log.Info().Msgf("Calling DeleteClusterMetadata on kuber for cluster %s", cluster.ClusterInfo.Name)
			_, err := kuber.DeleteClusterMetadata(c, &pb.DeleteClusterMetadataRequest{Cluster: cluster})
			return err
		})
	}

	return group.Wait()
}

// saveErrorMessage saves error message to config
func saveErrorMessage(config *pb.Config, c pb.ContextBoxServiceClient, err error) error {
	if config.DesiredState != nil {
		// Update currentState preemptively, so we can use it for terraform destroy
		// id DesiredState is null, we are already in deletion process, thus CurrentState should stay as is when error occurs
		config.CurrentState = config.DesiredState
	}
	config.ErrorMessage = err.Error()
	errSave := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
	if errSave != nil {
		return fmt.Errorf("error while saving the config in Builder: %w", err)
	}
	return nil
}
