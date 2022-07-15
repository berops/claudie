package main

import (
	"fmt"

	"github.com/Berops/platform/envs"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	kubeEleven "github.com/Berops/platform/services/kube-eleven/client"
	kuber "github.com/Berops/platform/services/kuber/client"
	terraformer "github.com/Berops/platform/services/terraformer/client"
	wireguardian "github.com/Berops/platform/services/wireguardian/client"
	"github.com/Berops/platform/utils"
	"github.com/rs/zerolog/log"
)

// buildConfig is function used to carry out task specific to Builder concurrently
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
	// call Wireguardian to build VPN
	desiredState, err = callWireguardian(config.GetDesiredState(), config.GetCurrentState())
	if err != nil {
		err1 := saveErrorMessage(config, c, err)
		if err1 != nil {
			return fmt.Errorf("error in Wireguardian: %v; unable to save error message config: %v", err, err1)
		}
		return fmt.Errorf("error in Wireguardian: %v", err)
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

func callWireguardian(desiredState, currenState *pb.Project) (*pb.Project, error) {
	cc, err := utils.GrpcDialWithInsecure("wireguardian", envs.WireguardianURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		utils.CloseClientConnection(cc)
		log.Info().Msgf("Closing the connection for wireguardian")
	}()
	// Creating the client
	c := pb.NewWireguardianServiceClient(cc)
	log.Info().Msgf("Calling RunAnsible on wireguardian")
	res, err := wireguardian.RunAnsible(c, &pb.RunAnsibleRequest{DesiredState: desiredState, CurrentState: currenState})
	if err != nil {
		return nil, err
	}

	return res.GetDesiredState(), nil
}

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

// function saveErrorMessage saves error message to config
func saveErrorMessage(config *pb.Config, c pb.ContextBoxServiceClient, err error) error {
	config.CurrentState = config.DesiredState // Update currentState, so we can use it for deletion later
	config.ErrorMessage = err.Error()
	errSave := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
	if errSave != nil {
		return fmt.Errorf("error while saving the config: %v", err)
	}
	return nil
}
