package terraformer

import (
	"context"
	"fmt"

	"github.com/Berops/platform/proto/pb"
	"github.com/rs/zerolog/log"
)

// BuildInfrastructure uses TerraformServiceClient to build/deploy the infrastructure
func BuildInfrastructure(c pb.TerraformerServiceClient, req *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	res, err := c.BuildInfrastructure(context.Background(), req) //sending request to the server and receiving response
	if err != nil {
		return nil, fmt.Errorf("error while calling BuildInfrastructure on Terraformer: %v", err)
	}

	log.Info().Msg("Infrastructure was successfully built")
	return res, nil
}

// DestroyInfrastructure uses TerraformServiceClient to destroy infrastructure
func DestroyInfrastructure(c pb.TerraformerServiceClient, req *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	res, err := c.DestroyInfrastructure(context.Background(), req)
	if err != nil {
		return res, fmt.Errorf("error while calling DestroyInfrastructure on Terraformer: %v", err)
	}

	log.Info().Msg("Infrastructure was successfully destroyed")
	return res, nil
}
