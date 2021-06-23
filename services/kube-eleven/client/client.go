package kubeEleven

import (
	"context"
	"github.com/Berops/platform/proto/pb"
	"log"
)

func BuildCluster(c pb.KubeElevenServiceClient, req *pb.BuildClusterRequest) (*pb.BuildClusterResponse, error) {
	res, err := c.BuildCluster(context.Background(), req)
	if err != nil {
		log.Fatalln(err)
	}
	return res, nil
}
