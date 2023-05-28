package usecases

import (
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v2"

	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

// If the workflow happens correctly, the desired state for the K8s cluster
// (represented by request.DesiredCluster) has already been reached in the kube-eleven microservice.
// Inside the K8s cluster, in the kube-public namespace there is a configmap named 'cluster-info'
// which holds the kubeconfig for this cluster.
// Currently, that kubeconfig represents the older state of this cluster.
// PatchClusterInfoConfigMap updates that kubeconfig so that it represents the already reached
// desired state of the cluster.
func (u *Usecases) PatchClusterInfoConfigMap(request *pb.PatchClusterInfoConfigMapRequest) (*pb.PatchClusterInfoConfigMapResponse, error) {
	logger := utils.CreateLoggerWithClusterName(utils.GetClusterID(request.DesiredCluster.ClusterInfo))

	logger.Info().Msgf("Patching configmap containg cluster-info")

	var err error
	defer func() {
		if err != nil {
			logger.Err(err).Msgf("Error while patching configmap containing cluster-info")
		} else {
			logger.Info().Msgf("Configmap containing cluster-info patched successfully")
		}
	}()

	k := kubectl.Kubectl{
		Kubeconfig: request.DesiredCluster.Kubeconfig,
	}

	// Extract kubeconfig from the desired state of the K8s cluster.
	// This kubeconfig is upto date with the desired state of the cluster (which has
	// already been achieved in the kube-eleven microservice).
	var desiredStateKubeconfig map[string]interface{}
	if err = yaml.Unmarshal([]byte(request.DesiredCluster.Kubeconfig), &desiredStateKubeconfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubeconfig present in desired state of the K8s cluster. Got malformed YAML.")
	}
	// Go to the clusters section of the kubeconfig.
	clustersInDesiredStateKubeconfig := desiredStateKubeconfig["clusters"].([]interface{})
	if len(clustersInDesiredStateKubeconfig) == 0 { // TODO: know about when this can happen
		return nil, fmt.Errorf("kubeconfig in desired state contains no K8s clusters")
	}
	// Get the cluster named "cluster" from the clusters section.
	/*
		desired state
		|
		|_kubeconfig
			|
			|_clusters
				|
				|_ cluster (represents request.DesiredCluster)
	*/
	clusterInDesiredStateKubeconfig := clustersInDesiredStateKubeconfig[0].(map[string]interface{})["cluster"].(map[string]interface{})

	// The configmap named "cluster-info" resides in "kube-public" namespace of the cluster.
	// It contains kubeconfig of that K8s cluster.
	// Fetching contents of the configmap (in JSON format).
	configmap, err := k.KubectlGet("cm cluster-info", "-ojson", "-n kube-public")
	if err != nil {
		return nil, err

		// TODO: know about the situation in which "configmap == nil" will happen.
	} else if configmap == nil {
		return &pb.PatchClusterInfoConfigMapResponse{}, nil
	}
	// Extract kubeconfig from the configmap.
	// This kubeconfig is not upto date with the desired state of the cluster (which has
	// already been achieved in the kube-eleven microservice).
	var configmapKubeconfig map[string]interface{}
	if err = yaml.Unmarshal(
		[]byte(gjson.Get(string(configmap), "data.kubeconfig").String()),
		&configmapKubeconfig,
	); err != nil {
		return nil, fmt.Errorf("failed to unmarshal kubeconfig present in 'cluster-info' configmap. Got malformed YAML")
	}
	// Go to the clusters section of the kubeconfig
	clustersInConfigmapKubeconfig := configmapKubeconfig["clusters"].([]interface{})
	if len(clustersInConfigmapKubeconfig) == 0 { // TODO: know about when this can happen
		return nil, fmt.Errorf("kubeconfig in configmap contains no K8s clusters")
	}
	// Get the cluster named "cluster" from the clusters section
	/*
		configmap
		|
		|_kubeconfig
			|
			|_clusters
				|
				|_ cluster (represents request.DesiredCluster)
	*/
	clusterInConfigmapKubeconfig := clustersInConfigmapKubeconfig[0].(map[string]interface{})["cluster"].(map[string]interface{})

	// Update clusterInConfigmapKubeconfig.
	clusterInConfigmapKubeconfig["server"] = clusterInDesiredStateKubeconfig["server"]
	clusterInConfigmapKubeconfig["certificate-authority-data"] = clusterInDesiredStateKubeconfig["certificate-authority-data"]

	// Save those updates in kubeconfig by patching the configmap.
	b, err := yaml.Marshal(configmapKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated kubeconfig (for configmap 'cluster-info')")
	}
	patchedConfigmap, err := sjson.Set(string(configmap), "data.kubeconfig", b)
	if err != nil {
		return nil, fmt.Errorf("failed to update configmap 'cluster-info' with updated kubeconfig")
	}
	if err = k.KubectlApplyString(patchedConfigmap, "-n kube-public"); err != nil {
		return nil, fmt.Errorf("failed to patch configmap 'cluster-info': %w", err)
	}

	return &pb.PatchClusterInfoConfigMapResponse{}, nil
}
