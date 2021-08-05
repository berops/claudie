package cbox

import (
	"context"
	"fmt"

	"github.com/Berops/platform/proto/pb"
)

// SaveConfigFrontEnd calls Content-box gRPC server and saves configuration to the mongoDB database
// A new config file with Id will be created if ID is empty
func SaveConfigFrontEnd(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	fmt.Println("Saving config")

	res, err := c.SaveConfigFrontEnd(context.Background(), req)
	if err != nil {
		return fmt.Errorf("unexpected error: %v", err)
	}

	fmt.Println("Config", res.GetConfig().GetName(), "has been saved")
	return nil
}

//SaveConfigScheduler saves config from Scheduler
func SaveConfigScheduler(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	fmt.Println("Saving config")

	res, err := c.SaveConfigScheduler(context.Background(), req)
	if err != nil {
		return fmt.Errorf("unexpected error: %v", err)
	}

	fmt.Println("Config", res.GetConfig().GetName(), "has been saved")
	return nil
}

//SaveConfigBuilder saves config from Scheduler
func SaveConfigBuilder(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	fmt.Println("Saving config")

	res, err := c.SaveConfigBuilder(context.Background(), req)
	if err != nil {
		return fmt.Errorf("unexpected error: %v", err)
	}

	fmt.Println("Config", res.GetConfig().GetName(), "has been saved")
	return nil
}

//GetConfigScheduler gets config from queueScheduler in which are available configs for Scheduler
func GetConfigScheduler(c pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	res, err := c.GetConfigScheduler(context.Background(), &pb.GetConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("unexpected error: %v", err)
	}

	return res, nil
}

//GetConfigBuilder gets config from queueBuilder in which are available configs for Builder
func GetConfigBuilder(c pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	res, err := c.GetConfigBuilder(context.Background(), &pb.GetConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("unexpected error: %v", err)
	}

	return res, nil
}

// GetAllConfigs calls Content-box gRPC server and returns all configs from the mongoDB database
func GetAllConfigs(c pb.ContextBoxServiceClient) (*pb.GetAllConfigsResponse, error) {
	res, err := c.GetAllConfigs(context.Background(), &pb.GetAllConfigsRequest{})
	if err != nil {
		return nil, fmt.Errorf("unexpected error: %v", err)
	}

	return res, nil
}

// DeleteConfig deletes object from the mongoDB database with a specified Id
func DeleteConfig(c pb.ContextBoxServiceClient, id string) error {
	res, err := c.DeleteConfig(context.Background(), &pb.DeleteConfigRequest{Id: id})
	if err != nil {
		return fmt.Errorf("error happened while deleting: %v", err)
	}

	fmt.Println("Config was deleted", res)
	return nil
}
