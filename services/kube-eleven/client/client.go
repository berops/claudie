package kubeEleven

import (
	"context"
	"log"

	"github.com/Berops/platform/proto/pb"
)

// BuildCluster uses KubeEleven service client to deploy our cluster
func BuildCluster(c pb.KubeElevenServiceClient, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	res, err := c.BuildCluster(context.Background(), req)
	if err != nil {
		return nil, err
	}
	log.Println("Clusters were successfully built")
	return res, nil
}
