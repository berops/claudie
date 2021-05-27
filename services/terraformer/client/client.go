package terraformer

import (
	"context"
	"fmt"
	"log"

	"github.com/Berops/platform/proto/pb"
)

func BuildInfrastructure(c pb.TerraformerServiceClient, req *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	fmt.Println("Sending a project message to Terraformer.")

	res, err := c.BuildInfrastructure(context.Background(), req) //sending request to the server and receiving response
	if err != nil {
		log.Fatalln("error while calling BuildInfrastructure on Terraformer", err)
	}
	log.Println("Infrastructure was successfully built", res)

	return res, nil
}
