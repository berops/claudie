package terraformer

import (
	"log"
	"testing"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestBuildInfrastructure(t *testing.T) {
	//Create connection to Terraformer
	cc, err := grpc.Dial(ports.TerraformerPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer func() {
		err := cc.Close()
		require.NoError(t, err)
	}()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)

	res, err := BuildInfrastructure(c, &pb.BuildInfrastructureRequest{
		Config: &pb.Config{
			Name: "Test config for Terraformer",
		},
	})
	require.NoError(t, err)
	t.Log("Terraformer response: ", res)
}
