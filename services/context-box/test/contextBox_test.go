package test

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestGetConfig(t *testing.T) {
	//Create connection to Context-box
	cc, err := grpc.Dial(ports.ContextBoxPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	res, err := cbox.GetConfig(c)
	require.NoError(t, err)
	t.Logf("ID                       Name\n")
	for _, c := range res.GetConfigs() {
		t.Log(c.GetId(), c.GetName(), c.GetDesiredState(), c.CurrentState)
	}
}

func TestSaveConfig(t *testing.T) {
	//Create connection to Context-box
	cc, err := grpc.Dial(ports.ContextBoxPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	manifest, errR := ioutil.ReadFile("manifest.yaml")
	if errR != nil {
		log.Fatalln(errR)
	}
	config := &pb.Config{
		//Id:       "6049f860a0cf8c4d391e6f58",
		Name:         "test_while_testing",
		Manifest:     string(manifest),
		DesiredState: &pb.Project{Name: "This is desiredState name"},
		CurrentState: &pb.Project{Name: "This is currentState name"},
	}

	err = cbox.SaveConfig(c, config)
	require.NoError(t, err)
}

func TestDeleteConfig(t *testing.T) {
	//Create connection to Context-box
	cc, err := grpc.Dial(ports.ContextBoxPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	err = cbox.DeleteConfig(c, "6049f856a0cf8c4d391e6f57")
	require.NoError(t, err)
}
