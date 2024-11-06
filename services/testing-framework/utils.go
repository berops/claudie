package testingframework

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb/spec"
	managerclient "github.com/berops/claudie/services/manager/client"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v3"

	"google.golang.org/protobuf/proto"
)

const (
	maxTimeout     = 24_500  // max allowed time for one manifest to finish in [seconds]
	sleepSec       = 10      // seconds for one cycle of config check
	maxTimeoutSave = 60 * 12 // max allowed time for config to be found in the database
)

var (
	errInterrupt = errors.New("interrupt")

	opts = cmpopts.IgnoreUnexported(
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
		spec.Events{},
		spec.TaskEvent{},
		spec.RetryStrategy{},
		spec.Task{},
		spec.CreateState{},
		spec.UpdateState{},
		spec.UpdateState_Endpoint{},
		spec.DeleteState{},
		spec.DeletedNodes{},
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
		spec.GenesisCloudProvider{},
		spec.Provider{},
		spec.Provider_Gcp{},
		spec.Provider_Hetzner{},
		spec.Provider_Hetznerdns{},
		spec.Provider_Oci{},
		spec.Provider_Aws{},
		spec.Provider_Azure{},
		spec.Provider_Cloudflare{},
		spec.Provider_Genesiscloud{},
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
			if res.Config.Manifest.State == spec.Manifest_Done {
				if bytes.Equal(res.Config.Manifest.LastAppliedChecksum, res.Config.Manifest.Checksum) {
					if err := validateKubeconfigAlternativeNames(res.Config.Clusters); err != nil {
						return nil, err
					}
					return res.Config, nil
				}

				// In case a test-set contains static nodepools and the test set performs
				// a rolling update the static pools needs to be placed first in the input manifest.
				// As a rolling update appends new nodepools and skips over static nodepool the
				// order between the current and desired state will be different and fails the
				// below check, but the end state does match
				for c, s := range res.Config.Clusters {
					equal := proto.Equal(s.Current, s.Desired)
					if !equal {
						diff := cmp.Diff(s.Current, s.Desired, opts)
						log.Debug().Msgf("cluster %q failed: %s", c, diff)
						return nil, fmt.Errorf("cluster %q has current state diverging from the desired state", c)
					}
				}
			}

			if res.Config.Manifest.State == spec.Manifest_Error && bytes.Equal(res.Config.Manifest.LastAppliedChecksum, res.Config.Manifest.Checksum) {
				var err error
				if validateErr := validateKubeconfigAlternativeNames(res.Config.Clusters); validateErr != nil {
					err = errors.Join(err, validateErr)
				}

				for cluster, state := range res.Config.Clusters {
					if state.State.Status == spec.Workflow_ERROR {
						err = errors.Join(err, fmt.Errorf("----\nerror in cluster %s\n----\nStage: %s \n State: %s\n Description: %s", cluster, state.State.Stage, state.State.Status, state.State.Description))
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
		if utils.IsAutoscaled(s.Current.GetK8S()) {
			clusters = append(clusters, s.Current.GetK8S())
		}
	}

	return clusters
}

func validateKubeconfigAlternativeNames(clusters map[string]*spec.ClusterState) error {
	for c, v := range clusters {
		if v.Current == nil || v.Current.K8S.Kubeconfig == "" {
			continue
		}
		// if the clusters has no APIServer Loadbalancer we can test all
		// control plane nodes to validate if they all can be used with the
		// generated KubeConfig.
		apiLb := false
		for _, l := range v.GetCurrent().GetLoadBalancers().GetClusters() {
			if utils.HasAPIServerRole(l.Roles) {
				apiLb = true
				break
			}
		}
		if apiLb {
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
