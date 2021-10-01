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

func ClientConnection() (pb.ContextBoxServiceClient, *grpc.ClientConn) {
	cc, err := grpc.Dial(urls.ContextBoxURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c, cc
}

func closeConn(t *testing.T, connection *grpc.ClientConn) {
	err := connection.Close()
	if err != nil {
		log.Fatalln("Error while closing the client connection:", err)
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

func makePbConfig(msg string, manifest []byte) *pb.Config {
	return &pb.Config{
		Name:     msg,
		Manifest: string(manifest),
	}
}
func TestSaveConfigFrontEnd(t *testing.T) {
	c, cc := ClientConnection()
	manifest, errR := ioutil.ReadFile("./manifest.yaml") //this is manifest from this test file
	if errR != nil {
		log.Fatalln(errR)
	}

	_, cfgErr := SaveConfigFrontEnd(c, &pb.SaveConfigRequest{
		Config: makePbConfig("TestDeleteConfig Samo", manifest),
	})
	if cfgErr != nil {
		log.Fatalln("Error saving FrontEnd configuration to DB connection:", cfgErr)
	}
	closeConn(t, cc)
}

func TestSaveConfigScheduler(t *testing.T) {
	c, cc := ClientConnection()

	manifest, errR := ioutil.ReadFile("./manifest.yaml") //this is manifest from this test file
	if errR != nil {
		log.Fatalln(errR)
	}

	cfgErr := SaveConfigScheduler(c, &pb.SaveConfigRequest{
		Config: makePbConfig("TestDeleteNodeSamo", manifest),
	})
	if cfgErr != nil {
		log.Fatalln("Error saving Scheduler configuration to DB connection:", cfgErr)
	}
	closeConn(t, cc)
}

func TestDeleteConfig(t *testing.T) {
	c, cc := ClientConnection()
	configID := "6126737f4f9bcdabaa336da4"
	delErr := DeleteConfig(c, configID)
	if delErr != nil {
		log.Fatalf("Error deleting config %s %s\n", configID, delErr)
	}
	closeConn(t, cc)
}

func TestPrintConfig(t *testing.T) {
	c, cc := ClientConnection()
	_, err := PrintConfig(c, "6126737f4f9bcdabaa336da4")
	if err != nil {
		log.Fatalln("Config not found:", err)
	}
	closeConn(t, cc)
}
