package main

import (
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"google.golang.org/grpc"
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
	Location   string `yaml:"location"`
	Datacenter string `yaml:"datacenter"`
}

type Worker struct {
	Count      int32  `yaml:"count"`
	ServerType string `yaml:"server_type"`
	Image      string `yaml:"image"`
	DiskSize   uint32 `yaml:"disk_size"`
	Zone       string `yaml:"zone"`
	Location   string `yaml:"location"`
	Datacenter string `yaml:"datacenter"`
}

////////////////////////////////////////////////////////////////////////////////

func createDesiredState(config *pb.Config) *pb.Config {
	//Create yaml manifest
	d := []byte(config.GetManifest())
	err := ioutil.WriteFile("manifest.yaml", d, 0644)
	if err != nil {
		log.Fatalln("Error while creating manifest.yaml file", err)
	}
	//Parse yaml to protobuf and create desiredState
	var desiredState Manifest
	yamlFile, err := ioutil.ReadFile("manifest.yaml")
	if err != nil {
		log.Fatalln("error while reading maninfest.yaml file", err)
	}
	err = yaml.Unmarshal(yamlFile, &desiredState)
	if err != nil {
		log.Fatalln("error while unmarshalling yaml file", err)
	}
	//Remove yaml manifest after loading
	err = os.Remove("manifest.yaml")
	if err != nil {
		log.Fatalln("error while removing maninfest.yaml file", err)
	}

	clusters := []*pb.Cluster{}
	for _, cluster := range desiredState.Clusters {
		nodePools := []*pb.NodePool{}
		for _, nodePool := range cluster.NodePools {
			nodePools = append(nodePools, &pb.NodePool{
				Name:   nodePool.Name,
				Region: nodePool.Region,
				Master: &pb.Node{
					Count:      uint32(nodePool.Master.Count),
					ServerType: nodePool.Master.ServerType,
					Image:      nodePool.Master.Image,
					DiskSize:   nodePool.Master.DiskSize,
					Zone:       nodePool.Master.Zone,
					Location:   nodePool.Master.Location,
					Datacenter: nodePool.Master.Datacenter,
				},
			})
		}

		clusters = append(clusters, &pb.Cluster{
			Name:       cluster.Name,
			Kubernetes: cluster.Kubernetes,
			Network:    cluster.Network,
			NodePools:  nodePools,
		})
	}

	return &pb.Config{
		Id:       config.GetId(),
		Name:     config.GetName(),
		Manifest: config.GetManifest(),
		DesiredState: &pb.Project{
			Name: desiredState.Name,
			Credentials: map[string]string{
				"gcp":     desiredState.PublicCloudCredentials.Gcp,
				"hetzner": desiredState.PublicCloudCredentials.Hetzner,
			},
			Clusters: clusters,
		},
	}
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

	//Infinite FOR cycle gets all configs from the context box every defined number of seconds
	for { // TODO: Maybe goroutines here?
		res, err := cbox.GetAllConfigs(c) //Get all configs from the database. It is a grpc call to the context-box
		if err != nil {
			log.Fatalln("Error while getting config from the Scheduler", err)
		}
		for _, config := range res.GetConfigs() {
			config = createDesiredState(config)
			//fmt.Println(config.GetDesiredState())
			err := cbox.SaveConfig(c, &pb.SaveConfigRequest{Config: config})
			if err != nil {
				log.Fatalln("Error while saving config", err)
			}
		}
		time.Sleep(5 * time.Second)
	}
}
