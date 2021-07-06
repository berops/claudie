package cbox

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/urls"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func ClientConnection() pb.ContextBoxServiceClient {
	cc, err := grpc.Dial(urls.ContextBoxURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c
}

func TestGetConfigScheduler(t *testing.T) {
	c := ClientConnection()

	res, err := GetConfigScheduler(c)
	require.NoError(t, err)
	t.Log("Config name", res.GetConfig().GetName())
}

func TestGetConfigBuilder(t *testing.T) {
	c := ClientConnection()

	res, err := GetConfigBuilder(c)
	require.NoError(t, err)
	t.Log("Config name", res.GetConfig().GetName())
}

func TestGetAllConfigs(t *testing.T) {
	c := ClientConnection()

	res, err := GetAllConfigs(c)
	require.NoError(t, err)
	for _, c := range res.GetConfigs() {
		t.Log(c.GetId(), c.GetName(), c.GetDesiredState(), c.CurrentState)
	}
}

func TestSaveConfigFrontEnd(t *testing.T) {
	c := ClientConnection()

	manifest, errR := ioutil.ReadFile("/Users/samuelstolicny/Github/Berops/platform/services/context-box/client/manifest.yaml") //this is manifest from this test file
	if errR != nil {
		log.Fatalln(errR)
	}

	err := SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
		Config: &pb.Config{
			Name:         "NewTestConfig name",
			Manifest:     string(manifest),
			DesiredState: &pb.Project{Name: "This is desiredState name"},
			CurrentState: &pb.Project{Name: "This is currentState name"},
		},
	})
	require.NoError(t, err)
}

func TestSaveConfigScheduler(t *testing.T) {
	c := ClientConnection()

	manifest, errR := ioutil.ReadFile("/Users/samuelstolicny/Github/Berops/platform/services/context-box/client/manifest.yaml") //this is manifest from this test file
	if errR != nil {
		log.Fatalln(errR)
	}

	err := SaveConfigScheduler(c, &pb.SaveConfigRequest{
		Config: &pb.Config{
			Id:           "60bf64e9489c76f2e72a768f",
			Name:         "TestSaveConfigScheduler",
			Manifest:     string(manifest),
			DesiredState: &pb.Project{Name: "This is desiredState name"},
			CurrentState: &pb.Project{Name: "This is currentState name"},
		},
	})
	require.NoError(t, err)
}

func TestDeleteConfig(t *testing.T) {
	c := ClientConnection()
	err := DeleteConfig(c, "60e40e3ab65e073bab1674ce")
	require.NoError(t, err)
}

func TestPrintConfig(t *testing.T) {
	c := ClientConnection()
	_, err := PrintConfig(c, "60e40e3ab65e073bab1674ce")
	if err != nil {
		log.Fatalln("Config not found:", err)
	}
	require.NoError(t, err)
}
