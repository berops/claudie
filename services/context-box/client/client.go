package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

func main() {
	cc, err := grpc.Dial(ports.ContextBoxPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	//saveConfig(c)
	//getConfig(c)
	//deleteConfig(c)
}

func saveConfig(c pb.ContextBoxServiceClient) {
	manifest, errR := ioutil.ReadFile("/Users/samuelstolicny/Github/Berops/platform/services/context-box/client/manifest.yaml")
	if errR != nil {
		log.Fatalln(errR)
	}

	fmt.Println("Saving config")
	config := &pb.Config{
		//Id:       "6049d7afc57394c1278f10a4",
		Name:     "test_without_states",
		Manifest: string(manifest),
		// DesiredState: &pb.Project{Name: "test"},
		// CurrentState: &pb.Project{Name: "test"},
	}
	res, err := c.SaveConfig(context.Background(), &pb.SaveConfigRequest{Config: config})
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	fmt.Println("Config", res.GetConfig().GetName(), "has been saved")
}

func getConfig(c pb.ContextBoxServiceClient) {
	res, err := c.GetConfig(context.Background(), &pb.GetConfigRequest{})
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	// Print config names and IDs
	fmt.Printf("ID                       Name\n")
	for _, c := range res.GetConfig() {
		fmt.Println(c.GetId(), c.GetName(), c.GetDesiredState(), c.CurrentState)
	}
}

func deleteConfig(c pb.ContextBoxServiceClient) {
	res, err := c.DeleteConfig(context.Background(), &pb.DeleteConfigRequest{Id: "6049d7afc57394c1278f10a4"})
	if err != nil {
		log.Fatalf("Error happened while deleting: %v\n", err)
	}
	fmt.Println("Config was deleted", res)
}
