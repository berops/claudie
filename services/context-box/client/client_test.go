package cbox

import (
	"os"
	"testing"

	"github.com/Berops/claudie/internal/envs"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const configIDDefault = "6228ab28e655d4721eae5727"

func ClientConnection() (pb.ContextBoxServiceClient, *grpc.ClientConn) {
	cc, err := utils.GrpcDialWithInsecure("context-box", envs.ContextBoxURL)
	if err != nil {
		log.Fatal().Err(err)
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

	manifest, errR := os.ReadFile(manifestFile)
	if errR != nil {
		log.Fatal().Msgf("Error reading file %s. %v", manifestFile, errR)
	}

	_, cfgErr := SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
		Config: makePbConfig("Sample config name", manifest, ""),
	})
	if cfgErr != nil {
		log.Fatal().Msgf("Error saving FrontEnd configuration to DB connection: %v", cfgErr)
	}
	closeConn(t, cc)
}

func TestSaveConfigScheduler(t *testing.T) {
	c, cc := ClientConnection()
	manifestFile := "./manifest.yaml" // this is manifest from this test file

	manifest, errR := os.ReadFile(manifestFile)
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
	configID := "636ce11237b549bf20be1c81" //configIDDefault // Put desired config ID here
	delErr := DeleteConfig(c, configID, pb.IdType_HASH)
	if delErr != nil {
		log.Fatal().Msgf("Error deleting config %s %v", configID, delErr)
	}
	closeConn(t, cc)
}

// To get an output of the test, run this from the test's directory: go test -timeout 30s -run ^TestPrintConfig$ github.com/Berops/claudie/services/context-box/client -v
func TestPrintConfig(t *testing.T) {
	c, cc := ClientConnection()
	out, err := printConfig(c, configIDDefault, pb.IdType_HASH) // Put desired config ID here
	if err != nil {
		log.Fatal().Msgf("Config not found: %v", err)
	}
	t.Log(out)
	closeConn(t, cc)
}
