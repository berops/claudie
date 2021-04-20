package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
)

////////////////////YAML STRUCT//////////////////////////////////////////////////

type Manifest struct {
	Name                   string                 `yaml:"name"`
	PublicCloudCredentials PublicCloudCredentials `yaml:"publicCloudCredentials"`
	Clusters               []Cluster              `yaml:"clusters"`
}

type PublicCloudCredentials struct {
	Gcp     string `yaml:"gcp"`
	Hetzner string `yaml:"hetzner"`
}

type Cluster struct {
	Name       string     `yaml:"name"`
	Kubernetes string     `yaml:"kubernetes"`
	Network    string     `yaml:"network"`
	NodePools  []NodePool `yaml:"nodePools"`
}

type NodePool struct {
	Name   string `yaml:"name"`
	Region string `yaml:"region"`
	Master Master `yaml:"master"`
	Worker Worker `yaml:"worker"`
}

type Master struct {
	Count      int32  `yaml:"count"`
	ServerType string `yaml:"server_type"`
	Image      string `yaml:"image"`
	DiskSize   uint32 `yaml:"disk_size"`
	Zone       string `yaml:"zone"`
}

type Worker struct {
	Count      int32  `yaml:"count"`
	ServerType string `yaml:"server_type"`
	Image      string `yaml:"image"`
	DiskSize   uint32 `yaml:"disk_size"`
	Zone       string `yaml:"zone"`
}

////////////////////////////////////////////////////////////////////////////////

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
		var desiredState Manifest
		yamlFile, err := ioutil.ReadFile("manifest.yaml")
		err = yaml.Unmarshal(yamlFile, &desiredState)
		if err != nil {
			return nil, err
		}
		log.Println(desiredState)

		//Remove yaml manifest after loading
		err = os.Remove("manifest.yaml")
		if err != nil {
			return nil, err
		}
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
		res, err := cbox.GetConfig(c) //Get all configs from the database. It is a grpc call.
		if err != nil {
			log.Fatalln("Error while getting config from the Scheduler", err)
		}
		createDesiredState(res)
		time.Sleep(5 * time.Second)
	}
}
