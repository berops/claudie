package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"gopkg.in/yaml.v3"
)

func (u *Usecases) PatchKubeProxyConfigMap(ctx context.Context, request *pb.PatchKubeProxyConfigMapRequest) (*pb.PatchKubeProxyConfigMapResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.DesiredCluster.ClusterInfo))
	logger.Info().Msgf("Patching kube-proxy ConfigMap")

	k := kubectl.Kubectl{
		Kubeconfig: request.DesiredCluster.Kubeconfig,
	}

	configMap, err := k.KubectlGet("cm kube-proxy", "-oyaml", "-n kube-system")
	if err != nil {
		return nil, err
	}

	if configMap == nil {
		return &pb.PatchKubeProxyConfigMapResponse{}, nil
	}

	var desiredKubeconfig map[string]interface{}
	if err := yaml.Unmarshal([]byte(request.DesiredCluster.Kubeconfig), &desiredKubeconfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubeconfig, malformed yaml : %w", err)
	}

	var rawConfigMap map[string]interface{}
	if err := yaml.Unmarshal(configMap, &rawConfigMap); err != nil {
		return nil, fmt.Errorf("failed to update cluster info config map, malformed yaml : %w", err)
	}

	// get the new api server address
	desiredCluster, err := extractClusterFromKubeconfig(desiredKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to extract cluster data from kubeconfing of desired state")
	}

	if err := getKubeconfigServerEndpoint(rawConfigMap, desiredCluster["server"]); err != nil {
		return nil, fmt.Errorf("failed to patch kube-proxy kubeconfig: %w", err)
	}

	b, err := yaml.Marshal(rawConfigMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patched config map : %w", err)
	}

	if err = k.KubectlApplyString(string(b), "-n kube-system"); err != nil {
		return nil, fmt.Errorf("failed to patch config map: %w", err)
	}

	// Delete old kube-proxy pods to use updated config-map
	if err := k.KubectlDeleteResource("pods", "-l k8s-app=kube-proxy", "-n kube-system"); err != nil {
		return nil, fmt.Errorf("failed to restart kube-proxy pods: %w", err)
	}

	logger.Info().Msgf("Kube-proxy Config Map patched successfully")
	return &pb.PatchKubeProxyConfigMapResponse{}, nil
}

func extractClusterFromKubeconfig(root map[string]any) (map[string]any, error) {
	clustersSlice, ok := root["clusters"].([]any)
	if !ok {
		return nil, errors.New("expected 'clusters' to be an []any")
	}
	if len(clustersSlice) == 0 {
		return nil, errors.New("no clusters to patch")
	}

	cluster, ok := clustersSlice[0].(map[string]any)
	if !ok {
		return nil, errors.New("expected clusters to be of type map[string]any")
	}

	clusterData, ok := cluster["cluster"].(map[string]any)
	if !ok {
		return nil, errors.New("expeted clusters of the kubeconfig to be of type map[string]any")
	}

	return clusterData, nil
}

func getKubeconfigServerEndpoint(root map[string]any, val any) error {
	data, ok := root["data"].(map[string]any)
	if !ok {
		return errors.New("expected 'data' field to be a map[string]any")
	}

	kubeConf, ok := data["kubeconfig.conf"].(string)
	if !ok {
		return errors.New("expected 'kubeconfig.conf' to be a string")
	}

	var kubeConfMap map[string]any
	if err := yaml.Unmarshal([]byte(kubeConf), &kubeConfMap); err != nil {
		return err
	}

	cluster, err := extractClusterFromKubeconfig(kubeConfMap)
	if err != nil {
		return err
	}

	cluster["server"] = val

	b, err := yaml.Marshal(kubeConfMap)
	if err != nil {
		return err
	}

	data["kubeconfig.conf"] = string(b)
	return nil
}
