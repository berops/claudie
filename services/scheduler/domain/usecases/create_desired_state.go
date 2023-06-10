package usecases

import (
	"fmt"

	"gopkg.in/yaml.v2"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/scheduler/utils"
)

// CreateDesiredState is a function which creates desired state of the project based on the unmarshalled manifest
// Returns *pb.Config for desired state if successful, error otherwise
func (u *Usecases) CreateDesiredState(config *pb.Config) (*pb.Config, error) {
	if config == nil {
		return nil, fmt.Errorf("CreateDesiredState got nil Config")
	}

	// Check if the manifest string is empty and set DesiredState to nil
	if config.Manifest == "" {
		return &pb.Config{
			Id:           config.GetId(),
			Name:         config.GetName(),
			Manifest:     config.GetManifest(),
			DesiredState: nil,
			CurrentState: config.GetCurrentState(),
			MsChecksum:   config.GetMsChecksum(),
			DsChecksum:   config.GetDsChecksum(),
			CsChecksum:   config.GetCsChecksum(),
			BuilderTTL:   config.GetBuilderTTL(),
			SchedulerTTL: config.GetSchedulerTTL(),
		}, nil
	}

	// Get the unmarshalled manifest
	unmarshalledManifest, err := getUnmarshalledManifest(config)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling manifest from config %s : %w ", config.Name, err)
	}

	// Construct desired state for clusters
	k8sClusters, err := utils.CreateK8sCluster(unmarshalledManifest)
	if err != nil {
		return nil, fmt.Errorf("error while creating kubernetes clusters for config %s : %w", config.Name, err)
	}
	lbClusters, err := utils.CreateLBCluster(unmarshalledManifest)
	if err != nil {
		return nil, fmt.Errorf("error while creating Loadbalancer clusters for config %s : %w", config.Name, err)
	}

	// Create new config for desired state
	newConfig := &pb.Config{
		Id:       config.GetId(),
		Name:     config.GetName(),
		Manifest: config.GetManifest(),
		DesiredState: &pb.Project{
			Name:                 unmarshalledManifest.Name,
			Clusters:             k8sClusters,
			LoadBalancerClusters: lbClusters,
		},
		CurrentState: config.GetCurrentState(),
		MsChecksum:   config.GetMsChecksum(),
		DsChecksum:   config.GetDsChecksum(),
		CsChecksum:   config.GetCsChecksum(),
		BuilderTTL:   config.GetBuilderTTL(),
		SchedulerTTL: config.GetSchedulerTTL(),
	}

	// Update info from current state into the desired state
	err = utils.UpdateK8sClusters(newConfig)
	if err != nil {
		return nil, fmt.Errorf("error while updating Kubernetes clusters for config %s : %w", config.Name, err)
	}
	err = utils.UpdateLBClusters(newConfig)
	if err != nil {
		return nil, fmt.Errorf("error while updating Loadbalancer clusters for config %s : %w", config.Name, err)
	}

	return newConfig, nil
}

// getUnmarshalledManifest will read manifest from the given config and return it in manifest.Manifest struct
// returns *manifest.Manifest if successful, error otherwise
func getUnmarshalledManifest(config *pb.Config) (*manifest.Manifest, error) {
	d := []byte(config.GetManifest())
	// Parse yaml to protobuf and create unmarshalledManifest
	var unmarshalledManifest manifest.Manifest
	err := yaml.Unmarshal(d, &unmarshalledManifest)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling yaml manifest for config %s: %w", config.Name, err)
	}
	return &unmarshalledManifest, nil
}
