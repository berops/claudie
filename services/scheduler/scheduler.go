package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type manifest struct {
	Name                   string `yaml:"name"`
	PublicCloudCredentials struct {
		Gcp     string `yaml:"gcp"`
		Hetzner string `yaml:"hetzner"`
		//TODO yaml parsing
	}
}

func createDesiredState(res *pb.GetConfigResponse) (*pb.SaveConfigRequest, error) {
	for _, config := range res.GetConfigs() { //res is slice of config
		//Create yaml manifest
		d := []byte(config.GetManifest())
		err := ioutil.WriteFile("manifest.yaml", d, 0644)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				fmt.Sprintf("Cannot create yaml manifest file: %v", err),
			)
		}
		//Parse yaml to protobuf and create desiredState
	}
	return nil, nil
}

func main() {
	//Create connection to Context-box
	cc, err := grpc.Dial(ports.ContextBoxPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	for { // TODO: Maybe goroutines here?
		res, err := cbox.GetConfig(c) //Get config from the database
		if err != nil {
			log.Fatalln("Error while getting config from the Scheduler", err)
		}
		createDesiredState(res)
		time.Sleep(5 * time.Second)
	}
}
