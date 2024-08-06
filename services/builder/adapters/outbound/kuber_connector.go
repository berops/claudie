package outbound

import (
	"github.com/berops/claudie/internal/envs"
	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	kuber "github.com/berops/claudie/services/kuber/client"
	"google.golang.org/grpc"
)

type KuberConnector struct {
	Connection *grpc.ClientConn
}

// Connect establishes a gRPC connection with the kuber microservice.
func (k *KuberConnector) Connect() error {
	connection, err := cutils.GrpcDialWithRetryAndBackoff("kuber", envs.KuberURL)
	if err != nil {
		return err
	}

	k.Connection = connection

	return nil
}

// SetUpStorage configures storage solution on given k8s cluster.
func (k *KuberConnector) SetUpStorage(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) (*pb.SetUpStorageResponse, error) {
	return kuber.SetUpStorage(kuberGrpcClient,
		&pb.SetUpStorageRequest{
			DesiredCluster: builderCtx.DesiredCluster,
		})
}

// StoreLBScrapeConfig stores LB scrape config on a given k8s cluster.
func (k *KuberConnector) StoreLBScrapeConfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.StoreLbScrapeConfig(kuberGrpcClient,
		&pb.StoreLBScrapeConfigRequest{
			Cluster:              builderCtx.DesiredCluster,
			DesiredLoadbalancers: builderCtx.DesiredLoadbalancers,
		})
}

// RemoveLBScrapeConfig removes LB scrape config from a given k8s cluster.
func (k *KuberConnector) RemoveLBScrapeConfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.RemoveLbScrapeConfig(kuberGrpcClient,
		&pb.RemoveLBScrapeConfigRequest{
			Cluster: builderCtx.DesiredCluster,
		})
}

// StoreClusterMetadata stores cluster metadata on a management k8s cluster.
func (k *KuberConnector) StoreClusterMetadata(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.StoreClusterMetadata(kuberGrpcClient,
		&pb.StoreClusterMetadataRequest{
			Cluster:       builderCtx.DesiredCluster,
			ProjectName:   builderCtx.ProjectName,
			Loadbalancers: builderCtx.DesiredLoadbalancers,
		})
}

// DeleteClusterMetadata removes cluster metadata from management k8s cluster.
func (k *KuberConnector) DeleteClusterMetadata(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.DeleteClusterMetadata(kuberGrpcClient,
		&pb.DeleteClusterMetadataRequest{
			Cluster: builderCtx.CurrentCluster,
		})
}

// StoreKubeconfig stores cluster kubeconfig on a management k8s cluster.
func (k *KuberConnector) StoreKubeconfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.StoreKubeconfig(kuberGrpcClient,
		&pb.StoreKubeconfigRequest{
			Cluster:     builderCtx.DesiredCluster,
			ProjectName: builderCtx.ProjectName,
		})
}

// DeleteKubeconfig removes cluster kubeconfig from management k8s cluster.
func (k *KuberConnector) DeleteKubeconfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.DeleteKubeconfig(kuberGrpcClient,
		&pb.DeleteKubeconfigRequest{
			Cluster: builderCtx.CurrentCluster,
		})
}

// SetUpClusterAutoscaler deploys cluster autoscaler on a management k8s cluster for a given k8s cluster.
func (k *KuberConnector) SetUpClusterAutoscaler(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.SetUpClusterAutoscaler(kuberGrpcClient,
		&pb.SetUpClusterAutoscalerRequest{
			ProjectName: builderCtx.ProjectName,
			Cluster:     builderCtx.DesiredCluster,
		})
}

// DestroyClusterAutoscaler deletes cluster autoscaler from a management k8s cluster for a given k8s cluster.
func (k *KuberConnector) DestroyClusterAutoscaler(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.DestroyClusterAutoscaler(kuberGrpcClient,
		&pb.DestroyClusterAutoscalerRequest{
			ProjectName: builderCtx.ProjectName,
			Cluster:     builderCtx.CurrentCluster,
		})
}

// PatchClusterInfoConfigMap updates certificates in a cluster-info config map for a given cluster.
func (k *KuberConnector) PatchClusterInfoConfigMap(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.PatchClusterInfoConfigMap(kuberGrpcClient,
		&pb.PatchClusterInfoConfigMapRequest{
			DesiredCluster: builderCtx.DesiredCluster,
		})
}

// PatchNodes updates k8s cluster node metadata.
func (k *KuberConnector) PatchNodes(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.PatchNodes(kuberGrpcClient,
		&pb.PatchNodesRequest{
			Cluster: builderCtx.DesiredCluster,
		})
}

// DeleteNodes gracefully removes nodes from a given k8s cluster.
func (k *KuberConnector) DeleteNodes(cluster *spec.K8Scluster, masterNodes, workerNodes []string, kuberGrpcClient pb.KuberServiceClient) (*pb.DeleteNodesResponse, error) {
	return kuber.DeleteNodes(kuberGrpcClient,
		&pb.DeleteNodesRequest{
			MasterNodes: masterNodes,
			WorkerNodes: workerNodes,
			Cluster:     cluster,
		})
}

func (k *KuberConnector) CiliumRolloutRestart(cluster *spec.K8Scluster, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.CiliumRolloutRestart(kuberGrpcClient,
		&pb.CiliumRolloutRestartRequest{
			Cluster: cluster,
		})
}

func (k *KuberConnector) PatchKubeProxyConfigMap(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.PatchKubeProxyConfigMap(kuberGrpcClient, &pb.PatchKubeProxyConfigMapRequest{
		DesiredCluster: builderCtx.DesiredCluster,
	})
}

// Disconnect closes the underlying gRPC connection to kuber microservice.
func (k *KuberConnector) Disconnect() {
	cutils.CloseClientConnection(k.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to kuber microservice.
func (k *KuberConnector) PerformHealthCheck() error {
	if err := cutils.IsConnectionReady(k.Connection); err == nil {
		return nil
	} else {
		k.Connection.Connect()
		return err
	}
}

// GetClient returns a kuber gRPC client.
func (k *KuberConnector) GetClient() pb.KuberServiceClient {
	return pb.NewKuberServiceClient(k.Connection)
}
