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

// Connect establishes a gRPC connection with the context-box microservice
func (k *KubeElevenConnector) Connect() error {
	connection, err := cutils.GrpcDialWithRetryAndBackoff("kube-eleven", envs.KubeElevenURL)
	if err != nil {
		return err
	}
	k.Connection = connection

	return nil
}

func (k *KubeElevenConnector) BuildCluster(builderCtx *utils.BuilderContext, kubeElevenGrpcClient pb.KubeElevenServiceClient) (*pb.BuildClusterResponse, error) {
	return kubeEleven.BuildCluster(kubeElevenGrpcClient,
		&pb.BuildClusterRequest{
			Desired:     builderCtx.DesiredCluster,
			DesiredLbs:  builderCtx.DesiredLoadbalancers,
			ProjectName: builderCtx.ProjectName,
		})
}

// Disconnect closes the underlying gRPC connection to context-box microservice
func (k *KubeElevenConnector) Disconnect() {
	cutils.CloseClientConnection(k.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to context-box microservice
func (k *KubeElevenConnector) PerformHealthCheck() error {
	return cutils.IsConnectionReady(k.Connection)
}

func (k *KubeElevenConnector) GetClient() pb.KubeElevenServiceClient {
	return pb.NewKubeElevenServiceClient(k.Connection)
}
