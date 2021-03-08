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

	// Save config
	content, errR := ioutil.ReadFile("/Users/samuelstolicny/Github/Berops/platform/services/context-box/client/config.yaml")
	if errR != nil {
		log.Fatalln(errR)
	}

	fmt.Println("Saving confing")
	config := &pb.Config{
		Id:      "6046125fe007b36dcb77b147",
		Name:    "test_created_edited",
		Content: string(content),
	}
	res, err := c.SaveConfig(context.Background(), &pb.SaveConfigRequest{Config: config})
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	fmt.Println("Config", res.GetConfig().GetName(), "has been saved")

}
