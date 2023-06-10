package usecases

import (
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb"
)

// createConfig generates and saves config into the DB. Used for new configs and updated configs.
func (u *Usecases) CreateConfig(inputManifest *manifest.Manifest) {
	inputManifestMarshalled, err := yaml.Marshal(inputManifest)
	if err != nil {
		log.Err(err).Msgf("Failed to marshal manifest %s. Skipping...", inputManifest.Name)
		return
	}

	// Define config
	config := &pb.Config{
		Name:             inputManifest.Name,
		ManifestFileName: inputManifest.Name,
		Manifest:         string(inputManifestMarshalled),
		State:            make(map[string]*pb.Workflow),
	}
	
	// build IN_PROGRES status for each cluster in the manifest
	for _, cluster := range inputManifest.Kubernetes.Clusters {
		config.State[cluster.Name] = &pb.Workflow{}
		config.State[cluster.Name].Status = pb.Workflow_IN_PROGRESS
	}

	if err := u.ContextBox.SaveConfig(config); err != nil {
		log.Err(err).Msgf("Failed to save config %v due to error. Skipping...", inputManifest.Name)
		return
	}
	log.Info().Msgf("Created config for input manifest %s", inputManifest.Name)

	// Put it into inProgress map to track it
	for _, k8sCluster := range inputManifest.Kubernetes.Clusters {
		if _, ok := u.inProgress.Load(k8sCluster.Name); !ok {
			u.inProgress.Store(k8sCluster.Name, config)
		}
	}
}

// deleteConfig generates and triggers deletion of config into the DB.
func (u *Usecases) DeleteConfig(inputManifest *manifest.Manifest) {

	if err := u.ContextBox.DeleteConfig(inputManifest.Name); err != nil {
		log.Err(err).Msgf("Failed to trigger deletion for config %v due to error. Skipping...", inputManifest.Name)
		return
	}

	log.Info().Msgf("Config %s was successfully marked for deletion", inputManifest.Name)

	// Put it into inProgress map to track it
	for _, k8sCluster := range inputManifest.Kubernetes.Clusters {
		if _, ok := u.inProgress.Load(k8sCluster.Name); !ok {
			// Use dummy config initially, it gets rewritten in new track cycle
			dummyConfig := &pb.Config{
				Name: inputManifest.Name,
			}
			u.inProgress.Store(k8sCluster.Name, dummyConfig)
		}
	}
}
