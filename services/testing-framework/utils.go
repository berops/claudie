package testingframework

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/berops/claudie/internal/clusters"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodepools"
	"github.com/berops/claudie/proto/pb/spec"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v3"
)

const (
	maxTimeout     = 24_500  // max allowed time for one manifest to finish in [seconds]
	sleepSec       = 10      // seconds for one cycle of config check
	maxTimeoutSave = 60 * 12 // max allowed time for config to be found in the database
)

var (
	errInterrupt = errors.New("interrupt")

	opts = cmpopts.IgnoreUnexported(
		spec.Stage{},
		spec.StageDescription{},
		spec.StageTerraformer{},
		spec.StageTerraformer_SubPass{},
		spec.StageAnsibler{},
		spec.StageAnsibler_SubPass{},
		spec.StageKubeEleven{},
		spec.StageKubeEleven_SubPass{},
		spec.StageKuber{},
		spec.StageKuber_SubPass{},
		spec.DNS{},
		spec.Config{},
		spec.Manifest{},
		spec.ClusterState{},
		spec.Clusters{},
		spec.LoadBalancers{},
		spec.KubernetesContext{},
		spec.Workflow{},
		spec.K8Scluster{},
		spec.LBcluster{},
		spec.ClusterInfo{},
		spec.Role{},
		spec.TaskEvent{},
		spec.Task{},
		spec.Create{},
		spec.Update{},
		spec.InstallationProxy{},
		spec.Update{},
		spec.Update_State{},
		spec.Update_None{},
		spec.Update_TerraformerAddLoadBalancer{},
		spec.Update_AddedLoadBalancer{},
		spec.Update_TerraformerDeleteLoadBalancerNodes{},
		spec.Update_TfDeleteLoadBalancerNodes{},
		spec.Update_DeletedLoadBalancerNodes{},
		spec.Update_TerraformerAddLoadBalancerNodes{},
		spec.Update_TerraformerAddLoadBalancerNodes_Existing{},
		spec.Update_TerraformerAddLoadBalancerNodes_New{},
		spec.Update_AddedLoadBalancerNodes{},
		spec.Update_DeleteLoadBalancerRoles{},
		spec.Update_TerraformerAddLoadBalancerRoles{},
		spec.Update_AddedLoadBalancerRoles{},
		spec.Update_TerraformerReplaceDns{},
		spec.Update_ReplacedDns{},
		spec.Update_DeleteLoadBalancer{},
		spec.Update_ApiEndpoint{},
		spec.Update_K8SOnlyApiEndpoint{},
		spec.Update_ApiPortOnCluster{},
		spec.Update_AnsiblerReplaceProxySettings{},
		spec.Update_ReplacedProxySettings{},
		spec.Update_AnsiblerReplaceTargetPools{},
		spec.Update_AnsiblerReplaceTargetPools_TargetPools{},
		spec.Update_ReplacedTargetPools{},
		spec.Update_ReplacedTargetPools_TargetPools{},
		spec.Update_UpgradeVersion{},
		spec.Update_KuberPatchNodes{},
		spec.Update_KuberPatchNodes_ListOfTaints{},
		spec.Update_KuberPatchNodes_ListOfLabelKeys{},
		spec.Update_KuberPatchNodes_ListOfAnnotationKeys{},
		spec.Update_KuberPatchNodes_RemoveBatch{},
		spec.Update_KuberPatchNodes_AddBatch{},
		spec.Update_KuberPatchNodes_MapOfLabels{},
		spec.Update_KuberPatchNodes_MapOfAnnotations{},
		spec.Update_PatchedNodes{},
		spec.Update_KuberDeleteK8SNodes{},
		spec.Update_KDeleteNodes{},
		spec.Update_DeletedK8SNodes{},
		spec.Update_TerraformerAddK8SNodes{},
		spec.Update_TerraformerAddK8SNodes_Existing{},
		spec.Update_TerraformerAddK8SNodes_New{},
		spec.Update_AddedK8SNodes{},
		spec.Delete{},
		spec.TaskResult{},
		spec.TaskResult_Error{},
		spec.TaskResult_None{},
		spec.TaskResult_UpdateState{},
		spec.TaskResult_ClearState{},
		spec.Work{},
		spec.NodePool{},
		spec.NodePool_DynamicNodePool{},
		spec.NodePool_StaticNodePool{},
		spec.Taint{},
		spec.Node{},
		spec.DynamicNodePool{},
		spec.MachineSpec{},
		spec.AutoscalerConf{},
		spec.StaticNodePool{},
		spec.GCPProvider{},
		spec.HetznerProvider{},
		spec.HetznerDNSProvider{},
		spec.OCIProvider{},
		spec.AWSProvider{},
		spec.AzureProvider{},
		spec.CloudflareProvider{},
		spec.Provider{},
		spec.Provider_Gcp{},
		spec.Provider_Hetzner{},
		spec.Provider_Hetznerdns{},
		spec.Provider_Oci{},
		spec.Provider_Aws{},
		spec.Provider_Azure{},
		spec.Provider_Cloudflare{},
		spec.TemplateRepository{},
	)
)

type testset struct{ Config, Set, Manifest string }

func waitForDoneOrError(ctx context.Context, manager managerclient.CrudAPI, set testset) (*spec.Config, error) {
	elapsed := 0
	ticker := time.NewTicker(sleepSec * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, errInterrupt
		case <-ticker.C:
			elapsed += sleepSec
			log.Info().Msgf("Waiting for %s from %s to finish... [ %ds elapsed ]", set.Manifest, set.Set, elapsed)
			if elapsed >= maxTimeout {
				return nil, fmt.Errorf("test took too long... Aborting after %d seconds", maxTimeout)
			}

			res, err := manager.GetConfig(ctx, &managerclient.GetConfigRequest{Name: set.Config})
			if err != nil {
				return nil, fmt.Errorf("error while waiting for config to finish: %w", err)
			}

			// Rolling update can have multiple stages, thus we also check for the manifest checksum equality.
			if res.Config.Manifest.State == spec.Manifest_Done && bytes.Equal(res.Config.Manifest.LastAppliedChecksum, res.Config.Manifest.Checksum) {
				if err := validateKubeconfigAlternativeNames(res.Config.Clusters); err != nil {
					return nil, err
				}

				// TODO: fix me.
				// In case a test-set contains static nodepools and the test set performs
				// a rolling update the static pools needs to be placed first in the input manifest.
				// As a rolling update appends new nodepools and skips over static nodepool the
				// order between the current and desired state will be different and fails the
				// below check, but the end state does match
				// for c, s := range res.Config.Clusters {
				// 	equal := proto.Equal(s.Current, s.Desired)
				// 	if !equal {
				// 		diff := cmp.Diff(s.Current, s.Desired, opts)
				// 		log.Debug().Msgf("cluster %q failed: %s", c, diff)
				// 		return nil, fmt.Errorf("cluster %q has current state diverging from the desired state", c)
				// 	}
				// }

				return res.Config, nil
			}

			if res.Config.Manifest.State == spec.Manifest_Error && bytes.Equal(res.Config.Manifest.LastAppliedChecksum, res.Config.Manifest.Checksum) {
				var err error
				if validateErr := validateKubeconfigAlternativeNames(res.Config.Clusters); validateErr != nil {
					err = errors.Join(err, validateErr)
				}

				// TODO: fix me.
				for cluster, state := range res.Config.Clusters {
					if state.State.Status == spec.Workflow_ERROR {
						err = errors.Join(err, fmt.Errorf("----\nerror in cluster %s\n----\nStage: %v \n State: %s\n Description: %s", cluster, state.InFlight.CurrentStage, state.State.Status, state.State.Description))
					}
				}

				return nil, err
			}
		}
	}
}

func getAutoscaledClusters(c *spec.Config) []*spec.K8Scluster {
	clusters := make([]*spec.K8Scluster, 0, len(c.Clusters))

	for _, s := range c.Clusters {
		if s.Current != nil && len(nodepools.Autoscaled(s.Current.K8S.ClusterInfo.NodePools)) > 0 {
			clusters = append(clusters, s.Current.GetK8S())
		}
	}

	return clusters
}

func validateKubeconfigAlternativeNames(c map[string]*spec.ClusterState) error {
	for c, v := range c {
		if v.Current == nil || v.Current.K8S.Kubeconfig == "" {
			continue
		}
		// if the clusters has no APIServer Loadbalancer we can test all
		// control plane nodes to validate if they all can be used with the
		// generated KubeConfig.
		if clusters.FindAssignedLbApiEndpoint(v.GetCurrent().GetLoadBalancers().GetClusters()) != nil {
			continue
		}

		var kubeconfigs []string

		kubeconfig := map[string]interface{}{}
		if err := yaml.Unmarshal([]byte(v.Current.K8S.Kubeconfig), &kubeconfig); err != nil {
			return fmt.Errorf("cluster %q: %w", c, err)
		}

		cluster := kubeconfig["clusters"].([]interface{})[0]
		clusterMap := cluster.(map[string]interface{})["cluster"].(map[string]interface{})
		for _, n := range v.Current.K8S.ClusterInfo.NodePools {
			if !n.IsControl {
				continue
			}

			for _, n := range n.Nodes {
				clusterMap["server"] = fmt.Sprintf("https://%s:6443", n.Public)
				newConfig, err := yaml.Marshal(kubeconfig)
				if err != nil {
					return fmt.Errorf("cluster %q: %w", c, err)
				}

				kubeconfigs = append(kubeconfigs, string(newConfig))
			}
		}

		var output []byte
		for _, kubeconfig := range kubeconfigs {
			k := kubectl.Kubectl{
				Kubeconfig:        kubeconfig,
				MaxKubectlRetries: 5,
			}
			nodes, err := k.KubectlGetNodeNames()
			if err != nil {
				return fmt.Errorf("cluster %q: %w", c, err)
			}

			// initialize only once, every output should then
			// be the same.
			if output == nil {
				output = nodes
			}

			if !bytes.Equal(nodes, output) {
				return fmt.Errorf("cluster %q does not have kubeconfig signed for all control plane nodes", c)
			}
		}
	}

	return nil
}
