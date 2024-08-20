package cbox

import (
	"context"
	"fmt"
	"reflect"
	"runtime"

	"google.golang.org/grpc"

	"github.com/berops/claudie/proto/pb"
)

type State string

// function to be used for saving
type saveFunction func(context.Context, *pb.SaveConfigRequest, ...grpc.CallOption) (*pb.SaveConfigResponse, error)

// SaveConfigOperator calls Content-box gRPC server and saves configuration to the database
// A new config file with Id will be created if ID is empty
// if successful, returns id of the saved config, error and empty string otherwise
func SaveConfigOperator(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) (string, error) {
	res, err := saveConfig(req, c.SaveConfigOperator)
	if err != nil {
		return "", err
	} else {
		return res.Config.Id, nil
	}
}

// SaveWorkflowState update the workflow state for a config.
func SaveWorkflowState(c pb.ContextBoxServiceClient, req *pb.SaveWorkflowStateRequest) error {
	_, err := c.SaveWorkflowState(context.Background(), req)
	return err
}

// SaveConfigScheduler saves config from Scheduler
func SaveConfigScheduler(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	_, err := saveConfig(req, c.SaveConfigScheduler)
	return err
}

// SaveConfigBuilder saves config from Builder
func SaveConfigBuilder(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	_, err := saveConfig(req, c.SaveConfigBuilder)
	return err
}

// GetConfigScheduler gets config from queueScheduler in which are available configs for Scheduler
func GetConfigScheduler(c pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	res, err := c.GetConfigScheduler(context.Background(), &pb.GetConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("error getting scheduler config: %w", err)
	}
	return res, nil
}

// GetTask gets the next task from the task queue.
func GetTask(c pb.ContextBoxServiceClient) (*pb.GetTaskResponse, error) {
	res, err := c.GetTask(context.Background(), &pb.GetTaskRequest{})
	if err != nil {
		return nil, fmt.Errorf("error getting builder config: %w", err)
	}
	return res, nil
}

// GetAllConfigs calls Content-box gRPC server and returns all configs from the mongoDB database
func GetAllConfigs(c pb.ContextBoxServiceClient) (*pb.GetAllConfigsResponse, error) {
	res, err := c.GetAllConfigs(context.Background(), &pb.GetAllConfigsRequest{})
	if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}
	return res, nil
}

// DeleteConfig sets the manifest to null so that the next invocation of the workflow
// for this config destroys the previous build infrastructure.
func DeleteConfig(c pb.ContextBoxServiceClient, req *pb.DeleteConfigRequest) error {
	if _, err := c.DeleteConfig(context.Background(), req); err != nil {
		return fmt.Errorf("error deleting: %w", err)
	}
	return nil
}

// DeleteConfigFromDB deletes the config from the mongoDB database.
func DeleteConfigFromDB(c pb.ContextBoxServiceClient, req *pb.DeleteConfigRequest) error {
	if _, err := c.DeleteConfigFromDB(context.Background(), req); err != nil {
		return fmt.Errorf("error deleting config from DB: %w", err)
	}
	return nil
}

func saveConfig(req *pb.SaveConfigRequest, saveFun saveFunction) (*pb.SaveConfigResponse, error) {
	res, err := saveFun(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to save config via %s : %w", runtime.FuncForPC(reflect.ValueOf(saveFun).Pointer()).Name() /*prints name of the function*/, err)
	}
	return res, nil
}
