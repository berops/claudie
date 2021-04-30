package cbox

import (
	"context"
	"fmt"
	"log"

	"github.com/Berops/platform/proto/pb"
)

// SaveConfigFrontEnd calls Content-box gRPC server and saves configuration to the mongoDB database
// A new config file with Id will be created if ID is empty
func SaveConfigFrontEnd(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	fmt.Println("Saving config")

	res, err := c.SaveConfigFrontEnd(context.Background(), req)
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	fmt.Println("Config", res.GetConfig().GetName(), "has been saved")
	return nil
}

//SaveConfigScheduler saves config from Scheduler
func SaveConfigScheduler(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	fmt.Println("Saving config")

	res, err := c.SaveConfigScheduler(context.Background(), req)
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	fmt.Println("Config", res.GetConfig().GetName(), "has been saved")
	return nil
}

func GetConfig(c pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	res, err := c.GetConfig(context.Background(), &pb.GetConfigRequest{})
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	return res, nil
}

// GetAllConfigs calls Content-box gRPC server and returns all configs from the mongoDB database
func GetAllConfigs(c pb.ContextBoxServiceClient) (*pb.GetAllConfigsResponse, error) {
	res, err := c.GetAllConfigs(context.Background(), &pb.GetAllConfigsRequest{})
	if err != nil {
		log.Fatalf("Unexpected error: %v", err)
	}
	return res, nil
}

// DeleteConfig deletes object from the mongoDB database with a specified Id
func DeleteConfig(c pb.ContextBoxServiceClient, id string) error {
	res, err := c.DeleteConfig(context.Background(), &pb.DeleteConfigRequest{Id: id})
	if err != nil {
		log.Fatalf("Error happened while deleting: %v\n", err)
	}
	fmt.Println("Config was deleted", res)
	return nil
}
