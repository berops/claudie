package terraformer

import (
	"context"
	"fmt"

	"github.com/berops/claudie/proto/pb"
)

// BuildInfrastructure uses TerraformServiceClient to build/deploy the infrastructure
func BuildInfrastructure(c pb.TerraformerServiceClient, req *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	res, err := c.BuildInfrastructure(context.Background(), req) //sending request to the server and receiving response
	if err != nil {
		return nil, fmt.Errorf("error while calling BuildInfrastructure on Terraformer: %w", err)
	}
	return res, nil
}

// DestroyInfrastructure uses TerraformServiceClient to destroy infrastructure
func DestroyInfrastructure(c pb.TerraformerServiceClient, req *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	res, err := c.DestroyInfrastructure(context.Background(), req)
	if err != nil {
		return res, fmt.Errorf("error while calling DestroyInfrastructure on Terraformer: %w", err)
	}
	return res, nil
}
