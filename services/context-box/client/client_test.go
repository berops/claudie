package cbox

import (
	"io/ioutil"
	"testing"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/urls"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func ClientConnection() (pb.ContextBoxServiceClient, *grpc.ClientConn) {
	cc, err := grpc.Dial(urls.ContextBoxURL, grpc.WithInsecure())
	if err != nil {
		log.Fatal().Msgf("Could not connect to server: %v", err)
	}

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c, cc
}

func closeConn(t *testing.T, connection *grpc.ClientConn) {
	err := connection.Close()
	if err != nil {
		log.Fatal().Msgf("Error while closing the client connection: %v", err)
	}
	require.NoError(t, err)
}

func TestGetConfigScheduler(t *testing.T) {
	c, cc := ClientConnection()

	res, _ := GetConfigScheduler(c)
	closeConn(t, cc)
	t.Log("Config name", res.GetConfig().GetName())
}

func TestGetConfigBuilder(t *testing.T) {
	c, cc := ClientConnection()

	res, _ := GetConfigBuilder(c)
	closeConn(t, cc)
	t.Log("Config name", res.GetConfig().GetName())
}

func TestGetAllConfigs(t *testing.T) {
	c, cc := ClientConnection()

	res, _ := GetAllConfigs(c)
	closeConn(t, cc)
	for _, c := range res.GetConfigs() {
		t.Log(c.GetId(), c.GetName(), c.GetDesiredState(), c.CurrentState)
	}
}

func makePbConfig(msg string, manifest []byte, id string) *pb.Config {
	return &pb.Config{
		Name:     msg,
		Manifest: string(manifest),
		Id:       id,
	}
}
func TestSaveConfigFrontEnd(t *testing.T) {
	c, cc := ClientConnection()
	manifestFile := "./manifest.yaml" // this is manifest from this test file

	manifest, errR := ioutil.ReadFile(manifestFile)
	if errR != nil {
		log.Fatal().Msgf("Error reading file %s. %v", manifestFile, errR)
	}

	_, cfgErr := SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
		Config: makePbConfig("TestConfig24", manifest, ""),
	})
	if cfgErr != nil {
		log.Fatal().Msgf("Error saving FrontEnd configuration to DB connection: %v", cfgErr)
	}
	closeConn(t, cc)
}

func TestSaveConfigScheduler(t *testing.T) {
	c, cc := ClientConnection()
	manifestFile := "./manifest.yaml" // this is manifest from this test file

	manifest, errR := ioutil.ReadFile(manifestFile)
	if errR != nil {
		log.Fatal().Msgf("Error reading file %s : %v", manifestFile, errR)
	}

	cfgErr := SaveConfigScheduler(c, &pb.SaveConfigRequest{
		Config: makePbConfig("TestDeleteNodeSamo", manifest, ""),
	})
	if cfgErr != nil {
		log.Fatal().Msgf("Error saving Scheduler configuration to DB connection: %v", cfgErr)
	}
	closeConn(t, cc)
}

func TestDeleteConfig(t *testing.T) {
	c, cc := ClientConnection()
	configID := "6126737f4f9bcdabaa336da4"
	delErr := DeleteConfig(c, configID)
	if delErr != nil {
		log.Fatal().Msgf("Error deleting config %s %v", configID, delErr)
	}
	closeConn(t, cc)
}

func TestPrintConfig(t *testing.T) {
	c, cc := ClientConnection()
	_, err := PrintConfig(c, "6126737f4f9bcdabaa336da4")
	if err != nil {
		log.Fatal().Msgf("Config not found: %v", err)
	}
	closeConn(t, cc)
}
