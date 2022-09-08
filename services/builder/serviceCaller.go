package main

import (
	"fmt"

	"github.com/Berops/claudie/internal/envs"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	ansibler "github.com/Berops/claudie/services/ansibler/client"
	cbox "github.com/Berops/claudie/services/context-box/client"
	kubeEleven "github.com/Berops/claudie/services/kube-eleven/client"
	kuber "github.com/Berops/claudie/services/kuber/client"
	terraformer "github.com/Berops/claudie/services/terraformer/client"
	"github.com/rs/zerolog/log"
)

// buildConfig is function used to build infra based on the desired state concurrently
func buildConfig(config *pb.Config, c pb.ContextBoxServiceClient, isTmpConfig bool) (err error) {
	log.Info().Msgf("processConfig received config: %s, is tmpConfig: %t", config.GetName(), isTmpConfig)
	// call Terraformer to build infra
	currentState, desiredState, err := callTerraformer(config.GetCurrentState(), config.GetDesiredState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in Terraformer: %v; unable to save error message config: %v", err, err1)
		}
		return fmt.Errorf("error in Terraformer: %v", err)
	}
	config.CurrentState = currentState
	config.DesiredState = desiredState
	// call Ansibler to build VPN
	desiredState, err = callAnsibler(config.GetDesiredState(), config.GetCurrentState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in Ansibler: %v; unable to save error message config: %v", err, err1)
		}
		return fmt.Errorf("error in Ansibler: %v", err)
	}
	config.DesiredState = desiredState
	// call Kube-eleven to create K8s clusters
	desiredState, err = callKubeEleven(config.GetDesiredState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in KubeEleven: %v; unable to save error message config: %v", err, err1)
		}
		return fmt.Errorf("error in KubeEleven: %v", err)
	}
	config.DesiredState = desiredState

	// call Kuber to set up longhorn
	desiredState, err = callKuber(config.GetDesiredState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in Kuber: %v; unable to save error message config: %v", err, err1)
		}
		return fmt.Errorf("error in Kuber: %v", err)
	}
	config.DesiredState = desiredState

	if !isTmpConfig {
		log.Info().Msgf("Saving the config %s", config.GetName())
		config.CurrentState = config.DesiredState // Update currentState
		err := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
		if err != nil {
			return fmt.Errorf("error while saving the config %s: %v", config.GetName(), err)
		}
	}

	return nil
}

//callTerraformer passes config to terraformer for building the infra
func callTerraformer(currentState *pb.Project, desiredState *pb.Project) (*pb.Project, *pb.Project, error) {
	// Create connection to Terraformer
	cc, err := utils.GrpcDialWithInsecure("terraformer", envs.TerraformerURL)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		utils.CloseClientConnection(cc)
		log.Info().Msgf("Closing the connection for terraformer")
	}()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	log.Info().Msgf("Calling BuildInfrastructure on terraformer")
	res, err := terraformer.BuildInfrastructure(c, &pb.BuildInfrastructureRequest{
		CurrentState: currentState,
		DesiredState: desiredState,
	})
	if err != nil {
		return nil, nil, err
	}

	return res.GetCurrentState(), res.GetDesiredState(), nil
}

//callAnsibler passes config to ansibler to set up VPN
func callAnsibler(desiredState, currentState *pb.Project) (*pb.Project, error) {
	cc, err := utils.GrpcDialWithInsecure("ansibler", envs.AnsiblerURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		utils.CloseClientConnection(cc)
		log.Info().Msgf("Closing the connection for ansibler")
	}()
	// Creating the client
	c := pb.NewAnsiblerServiceClient(cc)
	log.Info().Msgf("Calling InstallVPN on ansibler")
	installRes, err := ansibler.InstallVPN(c, &pb.InstallRequest{DesiredState: desiredState})
	if err != nil {
		return nil, err
	}
	log.Info().Msgf("Calling InstallNodeRequirements on ansibler")
	installRes, err = ansibler.InstallNodeRequirements(c, &pb.InstallRequest{DesiredState: installRes.DesiredState})
	if err != nil {
		return nil, err
	}
	log.Info().Msgf("Calling SetUpLoadbalancers on ansibler")
	setUpRes, err := ansibler.SetUpLoadbalancers(c, &pb.SetUpLBRequest{DesiredState: installRes.DesiredState, CurrentState: currentState})
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
	defer func() {
		utils.CloseClientConnection(cc)
		log.Info().Msgf("Closing the connection for kube-eleven")
	}()
	// Creating the client
	c := pb.NewKubeElevenServiceClient(cc)
	log.Info().Msgf("Calling BuildCluster on kube-eleven")
	res, err := kubeEleven.BuildCluster(c, &pb.BuildClusterRequest{DesiredState: desiredState})
	if err != nil {
		return nil, err
	}

	return res.GetDesiredState(), nil
}

//callKuber passes config to Kuber to apply any additional resources via kubectl
func callKuber(desiredState *pb.Project) (*pb.Project, error) {
	cc, err := utils.GrpcDialWithInsecure("kuber", envs.KuberURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		utils.CloseClientConnection(cc)
		log.Info().Msgf("Closing the connection for kuber")
	}()
	// Creating the client
	c := pb.NewKuberServiceClient(cc)
	log.Info().Msgf("Calling SetUpStorage on kuber")
	resStorage, err := kuber.SetUpStorage(c, &pb.SetUpStorageRequest{DesiredState: desiredState})
	if err != nil {
		return nil, err
	}
	for _, cluster := range desiredState.Clusters {
		log.Info().Msgf("Calling StoreKubeconfig on kuber")
		_, err := kuber.StoreKubeconfig(c, &pb.StoreKubeconfigRequest{Cluster: cluster})
		if err != nil {
			return nil, err
		}
	}
	return resStorage.GetDesiredState(), nil
}

//callDeleteNodes calls Kuber.DeleteNodes which will safely delete nodes from cluster
func callDeleteNodes(master, worker []string, cluster *pb.K8Scluster) (*pb.K8Scluster, error) {
	cc, err := utils.GrpcDialWithInsecure("kuber", envs.KuberURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		utils.CloseClientConnection(cc)
		log.Info().Msgf("Closing the connection for kuber")
	}()
	// Creating the client
	c := pb.NewKuberServiceClient(cc)
	log.Info().Msgf("Calling DeleteNodes on kuber")
	resDelete, err := kuber.DeleteNodes(c, &pb.DeleteNodesRequest{MasterNodes: master, WorkerNodes: worker, Cluster: cluster})
	if err != nil {
		return nil, err
	}
	return resDelete.Cluster, nil
}

// saveErrorMessage saves error message to config
func saveErrorMessage(config *pb.Config, c pb.ContextBoxServiceClient, err error) error {
	config.CurrentState = config.DesiredState // Update currentState, so we can use it for deletion later
	config.ErrorMessage = err.Error()
	errSave := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
	if errSave != nil {
		return fmt.Errorf("error while saving the config in Builder: %v", err)
	}
	return nil
}
