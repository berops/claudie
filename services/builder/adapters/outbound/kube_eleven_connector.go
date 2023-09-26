package outbound

import (
	"github.com/berops/claudie/internal/envs"
	cutils "github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/builder/domain/usecases/utils"
	kubeEleven "github.com/berops/claudie/services/kube-eleven/client"
	"google.golang.org/grpc"
)

type KubeElevenConnector struct {
	Connection *grpc.ClientConn
}

// Connect establishes a gRPC connection with the kube-eleven microservice
func (k *KubeElevenConnector) Connect() error {
	connection, err := cutils.GrpcDialWithRetryAndBackoff("kube-eleven", envs.KubeElevenURL)
	if err != nil {
		return err
	}
	k.Connection = connection

	return nil
}

// BuildCluster builds/reconciles given k8s cluster via kube-eleven.
func (k *KubeElevenConnector) BuildCluster(builderCtx *utils.BuilderContext, kubeElevenGrpcClient pb.KubeElevenServiceClient) (*pb.BuildClusterResponse, error) {
	return kubeEleven.BuildCluster(kubeElevenGrpcClient,
		&pb.BuildClusterRequest{
			Desired:     builderCtx.DesiredCluster,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

// DestroyCluster destroys k8s cluster.
func (k *KubeElevenConnector) DestroyCluster(builderCtx *utils.BuilderContext, kubeElevenGrpcClient pb.KubeElevenServiceClient) (*pb.DestroyClusterResponse, error) {
	return kubeEleven.DestroyCluster(kubeElevenGrpcClient,
		&pb.DestroyClusterRequest{
			ProjectName: builderCtx.ProjectName,
			Current:     builderCtx.CurrentCluster,
			CurrentLbs:  builderCtx.CurrentLoadbalancers,
		})
}

// Disconnect closes the underlying gRPC connection to kube-eleven microservice
func (k *KubeElevenConnector) Disconnect() {
	cutils.CloseClientConnection(k.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to kube-eleven microservice
func (k *KubeElevenConnector) PerformHealthCheck() error {
	return cutils.IsConnectionReady(k.Connection)
}

// GetClient returns a kube-eleven gRPC client.
func (k *KubeElevenConnector) GetClient() pb.KubeElevenServiceClient {
	return pb.NewKubeElevenServiceClient(k.Connection)
}
