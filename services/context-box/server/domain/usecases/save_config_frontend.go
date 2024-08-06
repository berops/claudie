package usecases

import (
	"fmt"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/context-box/server/utils"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v3"
)

// SaveConfigOperator saves config to MongoDB after receiving it from the Operator microservice
func (u *Usecases) SaveConfigOperator(request *pb.SaveConfigRequest) (*pb.SaveConfigResponse, error) {
	// Input specs can be changed by 2 entities - by Autoscaler or by User. There is a possibility that both of them can do it
	// at the same time. Thus, we need to lock the config while one entity updates it in the database, so the other entity does
	// not overwrite it.
	u.configChangeMutex.Lock()
	defer u.configChangeMutex.Unlock()

	newConfig := request.GetConfig()
	log.Info().Msgf("Saving config %s from claudie-operator", newConfig.Name)

	newConfig.MsChecksum = utils.CalculateChecksum(newConfig.Manifest)

	// Check if config with this name already exists in MongoDB
	oldConfig, err := u.DB.GetConfig(newConfig.GetName(), pb.IdType_NAME)
	if err == nil {
		if string(oldConfig.MsChecksum) != string(newConfig.MsChecksum) {
			oldConfig.Manifest = newConfig.Manifest
			oldConfig.MsChecksum = newConfig.MsChecksum
			oldConfig.SchedulerTTL = 0
			oldConfig.BuilderTTL = 0
			// clear error states (if any), to push the changed config into the workflow again.
			for cluster, wf := range oldConfig.State {
				if wf.Status == spec.Workflow_ERROR {
					oldConfig.State[cluster] = &spec.Workflow{}
				}
			}
		}
		newConfig = oldConfig
	}

	// Check if the new config has reference to already existing static nodes.
	configs, err := u.DB.GetAllConfigs()
	if err != nil {
		return nil, fmt.Errorf("failed to list stored configs")
	}
	if err := validateStaticNodepools(newConfig, configs); err != nil {
		return nil, fmt.Errorf("error while verifying static nodepools: %w", err)
	}

	if err = u.DB.SaveConfig(newConfig); err != nil {
		return nil, fmt.Errorf("error while saving config %s in MongoDB: %w", newConfig.Name, err)
	}

	log.Info().Msgf("Config %s successfully saved from claudie-operator", newConfig.Name)

	return &pb.SaveConfigResponse{Config: newConfig}, nil
}

func validateStaticNodepools(refConf *spec.Config, other []*spec.Config) error {
	refManifest, err := unmarshallManifest(refConf)
	if err != nil {
		return err
	}

	refManifestStaticIPs := collectIPs(refManifest)

	for _, cfg := range other {
		if cfg.Name == refConf.Name {
			continue
		}

		otherManifest, err := unmarshallManifest(cfg)
		if err != nil {
			return err
		}

		otherManifestStaticIPs := collectIPs(otherManifest)

		if match := findMatch(refManifestStaticIPs, otherManifestStaticIPs); match != "" {
			return fmt.Errorf("reference to the same static node with IP %q referenced in the newly added config %q is already in use by config %q, reference to the same static node across different clusters/configs is discouraged as it can lead to corrupt state of the cluster", match, refConf.Name, cfg.Name)
		}
	}

	return nil
}

func findMatch(first, second map[string]struct{}) string {
	for checkIP := range first {
		if _, ok := second[checkIP]; ok {
			return checkIP
		}
	}
	return ""
}

func collectIPs(m *manifest.Manifest) map[string]struct{} {
	nodepools := make(map[string]struct{})

	for _, snp := range m.NodePools.Static {
		for _, node := range snp.Nodes {
			nodepools[node.Endpoint] = struct{}{}
		}
	}

	return nodepools
}

func unmarshallManifest(config *spec.Config) (*manifest.Manifest, error) {
	d := []byte(config.GetManifest())

	var m manifest.Manifest
	if err := yaml.Unmarshal(d, &m); err != nil {
		return nil, fmt.Errorf("error while unmarshalling manifest for config %s: %w", config.Name, err)
	}

	return &m, nil
}
