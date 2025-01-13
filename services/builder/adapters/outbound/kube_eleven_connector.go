package outbound

import (
	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/grpcutils"
	"github.com/berops/claudie/proto/pb"
	builder "github.com/berops/claudie/services/builder/internal"
	kubeEleven "github.com/berops/claudie/services/kube-eleven/client"
	"google.golang.org/grpc"
)

type KubeElevenConnector struct {
	Connection *grpc.ClientConn
}

// Connect establishes a gRPC connection with the kube-eleven microservice
func (k *KubeElevenConnector) Connect() error {
	connection, err := grpcutils.GrpcDialWithRetryAndBackoff("kube-eleven", envs.KubeElevenURL)
	if err != nil {
		return err
	}

	k.Connection = connection

	return nil
}

// BuildCluster builds/reconciles given k8s cluster via kube-eleven.
func (k *KubeElevenConnector) BuildCluster(builderCtx *builder.Context, loadBalancerEndpoint string, kubeElevenGrpcClient pb.KubeElevenServiceClient) (*pb.BuildClusterResponse, error) {
	return kubeEleven.BuildCluster(kubeElevenGrpcClient,
		&pb.BuildClusterRequest{
			Desired:     builderCtx.DesiredCluster,
			LoadBalancerEndpoint:  loadBalancerEndpoint,
			ProxyEnvs:   builderCtx.ProxyEnvs,
			ProjectName: builderCtx.ProjectName,
		})
}

// DestroyCluster destroys k8s cluster.
func (k *KubeElevenConnector) DestroyCluster(builderCtx *builder.Context, loadBalancerEndpoint string, kubeElevenGrpcClient pb.KubeElevenServiceClient) (*pb.DestroyClusterResponse, error) {
	return kubeEleven.DestroyCluster(kubeElevenGrpcClient,
		&pb.DestroyClusterRequest{
			ProjectName:        builderCtx.ProjectName,
			Current:            builderCtx.CurrentCluster,
			LoadBalancerEndpoint: loadBalancerEndpoint,
		})
}

// Disconnect closes the underlying gRPC connection to kube-eleven microservice
func (k *KubeElevenConnector) Disconnect() {
	grpcutils.CloseClientConnection(k.Connection)
}

// PerformHealthCheck checks health of the underlying gRPC connection to kube-eleven microservice
func (k *KubeElevenConnector) PerformHealthCheck() error {
	if err := grpcutils.IsConnectionReady(k.Connection); err == nil {
		return nil
	} else {
		k.Connection.Connect()
		return err
	}
}

// GetClient returns a kube-eleven gRPC client.
func (k *KubeElevenConnector) GetClient() pb.KubeElevenServiceClient {
	return pb.NewKubeElevenServiceClient(k.Connection)
}
