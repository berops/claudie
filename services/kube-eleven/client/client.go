package kubeEleven

import (
	"context"
	"log"

	"github.com/Berops/platform/proto/pb"
)

func BuildCluster(c pb.KubeElevenServiceClient, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	res, err := c.BuildCluster(context.Background(), req)
	if err != nil {
		return nil, err
	}
	log.Println("Clusters were successfully built")
	return res, nil
}
