package terraformer

import (
	"context"
	"log"

	"github.com/Berops/platform/proto/pb"
)

func BuildInfrastructure(c pb.TerraformerServiceClient, req *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	res, err := c.BuildInfrastructure(context.Background(), req) //sending request to the server and receiving response
	if err != nil {
		log.Fatalln("error while calling BuildInfrastructure on Terraformer", err)
	}
	log.Println("Infrastructure was successfully built")
	return res, nil
}

func DestroyInfrastructure(c pb.TerraformerServiceClient, req *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	res, err := c.DestroyInfrastructure(context.Background(), req)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Infrastructure was successfully built destroyed")
	return res, nil
}
