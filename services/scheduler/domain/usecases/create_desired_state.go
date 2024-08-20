package usecases

import (
	"fmt"

	"github.com/berops/claudie/internal/manifest"
	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/scheduler/utils"
	"gopkg.in/yaml.v3"
)

// CreateDesiredState is a function which creates desired state of the project based on the unmarshalled manifest
// Returns *pb.Config for desired state if successful, error otherwise
func (u *Usecases) CreateDesiredState(config *spec.Config) (*spec.Config, error) {
	if config == nil {
		return nil, fmt.Errorf("CreateDesiredState got nil Config")
	}

	if config.Manifest == "" {
		return &spec.Config{
			Id:                config.GetId(),
			Name:              config.GetName(),
			ResourceName:      config.GetResourceName(),
			ResourceNamespace: config.GetResourceNamespace(),
			Manifest:          config.GetManifest(),
			DesiredState:      nil,
			CurrentState:      config.GetCurrentState(),
			MsChecksum:        config.GetMsChecksum(),
			DsChecksum:        config.GetDsChecksum(),
			CsChecksum:        config.GetCsChecksum(),
			BuilderTTL:        config.GetBuilderTTL(),
			SchedulerTTL:      config.GetSchedulerTTL(),
		}, nil
	}

	unmarshalledManifest, err := getUnmarshalledManifest(config)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling manifest from config %s : %w ", config.Name, err)
	}
	k8sClusters, err := utils.CreateK8sCluster(unmarshalledManifest)
	if err != nil {
		return nil, fmt.Errorf("error while creating kubernetes clusters for config %s : %w", config.Name, err)
	}
	lbClusters, err := utils.CreateLBCluster(unmarshalledManifest)
	if err != nil {
		return nil, fmt.Errorf("error while creating Loadbalancer clusters for config %s : %w", config.Name, err)
	}

	newConfig := &spec.Config{
		Id:                config.GetId(),
		Name:              config.GetName(),
		ResourceName:      config.GetResourceName(),
		ResourceNamespace: config.GetResourceNamespace(),
		Manifest:          config.GetManifest(),
		DesiredState: &spec.Project{
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

	// TODO: reimplement.
	if err := fixUpDuplicates(newConfig); err != nil {
		return nil, fmt.Errorf("failed to fixup duplicates for config %s: %w", config.Name, err)
	}
	if err := utils.UpdateK8sClusters(newConfig); err != nil {
		return nil, fmt.Errorf("error while updating Kubernetes clusters for config %s : %w", config.Name, err)
	}
	if err := utils.UpdateLBClusters(newConfig); err != nil {
		return nil, fmt.Errorf("error while updating Loadbalancer clusters for config %s : %w", config.Name, err)
	}

	return newConfig, nil
}

// getUnmarshalledManifest will read manifest from the given config and return it in manifest.Manifest struct
// returns *manifest.Manifest if successful, error otherwise
func getUnmarshalledManifest(config *spec.Config) (*manifest.Manifest, error) {
	d := []byte(config.GetManifest())
	// Parse yaml to protobuf and create unmarshalledManifest
	var unmarshalledManifest manifest.Manifest
	if err := yaml.Unmarshal(d, &unmarshalledManifest); err != nil {
		return nil, fmt.Errorf("error while unmarshalling yaml manifest for config %s: %w", config.Name, err)
	}
	return &unmarshalledManifest, nil
}

// fixUpDuplicates renames the nodepools if they're referenced multiple times in k8s,lb clusters.
func fixUpDuplicates(config *spec.Config) error {
	m, err := getUnmarshalledManifest(config)
	if err != nil {
		return err
	}

	clusterView := cutils.NewClusterView(config)
	for _, cluster := range clusterView.AllClusters() {
		desired := clusterView.DesiredClusters[cluster]
		desiredLbs := clusterView.DesiredLoadbalancers[cluster]

		current := clusterView.CurrentClusters[cluster]
		currentLbs := clusterView.Loadbalancers[cluster]

		for _, np := range m.NodePools.Dynamic {
			used := make(map[string]struct{})

			utils.CopyK8sNodePoolsNamesFromCurrentState(used, np.Name, current, desired)
			utils.CopyLbNodePoolNamesFromCurrentState(used, np.Name, currentLbs, desiredLbs)

			references := utils.FindNodePoolReferences(np.Name, desired.GetClusterInfo().GetNodePools())
			for _, lb := range desiredLbs {
				references = append(references, utils.FindNodePoolReferences(np.Name, lb.GetClusterInfo().GetNodePools())...)
			}

			for _, np := range references {
				hash := cutils.CreateHash(cutils.HashLength)
				for _, ok := used[hash]; ok; {
					hash = cutils.CreateHash(cutils.HashLength)
				}
				used[hash] = struct{}{}
				np.Name += fmt.Sprintf("-%s", hash)
			}
		}
	}

	return nil
}
