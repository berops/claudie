package client

import (
	"testing"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func ClientConnection() (pb.OperatorServiceClient, *grpc.ClientConn) {
	cc, err := utils.GrpcDialWithRetryAndBackoff("claudie-operator", envs.OperatorURL)
	if err != nil {
		log.Fatal().Err(err)
	}

	// Creating the client
	c := pb.NewOperatorServiceClient(cc)
	return c, cc
}

func closeConn(t *testing.T, connection *grpc.ClientConn) {
	err := connection.Close()
	if err != nil {
		log.Fatal().Msgf("Error while closing the client connection: %v", err)
	}
	require.NoError(t, err)
}

func TestSendAutoscalerEvent(t *testing.T) {
	c, cc := ClientConnection()

	testEvent := &pb.SendAutoscalerEventRequest{
		InputManifestName:      "test",
		InputManifestNamespace: "default",
	}

	err := SendAutoscalerEvent(c, testEvent)
	if err != nil {
		log.Fatal().Msgf("error: %s", err.Error())
	}
	closeConn(t, cc)
	t.Log("Event sent successfully")
}
