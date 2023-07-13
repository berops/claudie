package outbound

import (
	"github.com/berops/claudie/internal/envs"
	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	kuber "github.com/berops/claudie/services/kuber/client"
	"google.golang.org/grpc"
)

type KuberConnector struct {
	Connection *grpc.ClientConn
}

// Connect establishes a gRPC connection with the context-box microservice
func (k *KuberConnector) Connect() error {
	connection, err := cutils.GrpcDialWithRetryAndBackoff("kuber", envs.KuberURL)
	if err != nil {
		return err
	}
	k.Connection = connection

	return nil
}

func (k *KuberConnector) SetUpStorage(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) (*pb.SetUpStorageResponse, error) {
	return kuber.SetUpStorage(kuberGrpcClient,
		&pb.SetUpStorageRequest{
			DesiredCluster: builderCtx.DesiredCluster,
		})
}

func (k *KuberConnector) StoreLBScrapeConfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.StoreLbScrapeConfig(kuberGrpcClient,
		&pb.StoreLBScrapeConfigRequest{
			Cluster:              builderCtx.DesiredCluster,
			DesiredLoadbalancers: builderCtx.DesiredLoadbalancers,
		})
}

func (k *KuberConnector) RemoveLBScrapeConfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.RemoveLbScrapeConfig(kuberGrpcClient,
		&pb.RemoveLBScrapeConfigRequest{
			Cluster: builderCtx.DesiredCluster,
		})
}

func (k *KuberConnector) StoreClusterMetadata(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.StoreClusterMetadata(kuberGrpcClient,
		&pb.StoreClusterMetadataRequest{
			Cluster:     builderCtx.DesiredCluster,
			ProjectName: builderCtx.ProjectName,
		})
}

func (k *KuberConnector) DeleteClusterMetadata(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.DeleteClusterMetadata(kuberGrpcClient,
		&pb.DeleteClusterMetadataRequest{
			Cluster: builderCtx.CurrentCluster,
		})
}

func (k *KuberConnector) StoreKubeconfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.StoreKubeconfig(kuberGrpcClient,
		&pb.StoreKubeconfigRequest{
			Cluster:     builderCtx.DesiredCluster,
			ProjectName: builderCtx.ProjectName,
		})
}

func (k *KuberConnector) DeleteKubeconfig(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.DeleteKubeconfig(kuberGrpcClient,
		&pb.DeleteKubeconfigRequest{
			Cluster: builderCtx.CurrentCluster,
		})
}

func (k *KuberConnector) SetUpClusterAutoscaler(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.SetUpClusterAutoscaler(kuberGrpcClient,
		&pb.SetUpClusterAutoscalerRequest{
			ProjectName: builderCtx.ProjectName,
			Cluster:     builderCtx.DesiredCluster,
		})
}

func (k *KuberConnector) DestroyClusterAutoscaler(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.DestroyClusterAutoscaler(kuberGrpcClient,
		&pb.DestroyClusterAutoscalerRequest{
			ProjectName: builderCtx.ProjectName,
			Cluster:     builderCtx.CurrentCluster,
		})
}

func (k *KuberConnector) PatchClusterInfoConfigMap(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.PatchClusterInfoConfigMap(kuberGrpcClient,
		&pb.PatchClusterInfoConfigMapRequest{
			DesiredCluster: builderCtx.DesiredCluster,
		})
}

func (k *KuberConnector) PatchNodes(builderCtx *utils.BuilderContext, kuberGrpcClient pb.KuberServiceClient) error {
	return kuber.PatchNodes(kuberGrpcClient,
		&pb.PatchNodesRequest{
			Cluster: builderCtx.DesiredCluster,
		})
}

func (k *KuberConnector) DeleteNodes(cluster *pb.K8Scluster, masterNodes, workerNodes []string, kuberGrpcClient pb.KuberServiceClient) (*pb.DeleteNodesResponse, error) {
	return kuber.DeleteNodes(kuberGrpcClient,
		&pb.DeleteNodesRequest{
			MasterNodes: masterNodes,
			WorkerNodes: workerNodes,
			Cluster:     cluster,
		})
}

// Disconnect closes the underlying gRPC connection to context-box microservice
func (k *KuberConnector) Disconnect() {
	cutils.CloseClientConnection(k.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to context-box microservice
func (k *KuberConnector) PerformHealthCheck() error {
	return cutils.IsConnectionReady(k.Connection)
}

func (k *KuberConnector) GetClient() pb.KuberServiceClient {
	return pb.NewKuberServiceClient(k.Connection)
}
