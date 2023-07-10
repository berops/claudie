package cbox

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"runtime"

	"google.golang.org/grpc"

	"github.com/berops/claudie/proto/pb"
)

type State string

const (
	desired State = "DESIRED"
	current State = "CURRENT"
)

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

// GetConfigBuilder gets config from queueBuilder in which are available configs for Builder
func GetConfigBuilder(c pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	res, err := c.GetConfigBuilder(context.Background(), &pb.GetConfigRequest{})
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

func UpdateNodepoolCount(c pb.ContextBoxServiceClient, req *pb.UpdateNodepoolRequest) (*pb.UpdateNodepoolResponse, error) {
	return c.UpdateNodepool(context.Background(), req)
}

// printConfig prints a desired config with a current state info
func printConfig(c pb.ContextBoxServiceClient, id string, idType pb.IdType, state State) (string, error) {
	var buffer bytes.Buffer
	var printState *pb.Project
	res, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{Id: id, Type: idType})
	if err != nil {
		return "", fmt.Errorf("failed to get config ID %s : %w", id, err)
	}
	if state == desired {
		printState = res.GetConfig().GetDesiredState()
	} else {
		printState = res.GetConfig().GetCurrentState()
	}
	buffer.WriteString(fmt.Sprintf("\nConfig name: %s\n", res.GetConfig().GetName()))
	buffer.WriteString(fmt.Sprintf("Config ID: %s\n", res.GetConfig().GetId()))
	buffer.WriteString(fmt.Sprintf("Project name: %s\n", printState.GetName()))
	buffer.WriteString("Project clusters: \n")
	for i, cluster := range printState.GetClusters() {
		buffer.WriteString("========================================\n")
		buffer.WriteString(fmt.Sprintf("Cluster number: %d\n", i))
		buffer.WriteString(fmt.Sprintf("Name: %s\n", cluster.ClusterInfo.GetName()))
		buffer.WriteString(fmt.Sprintf("Hash: %s\n", cluster.ClusterInfo.GetHash()))
		buffer.WriteString(fmt.Sprintf("Kubernetes version: %s\n", cluster.GetKubernetes()))
		buffer.WriteString(fmt.Sprintf("Network CIDR: %s\n", cluster.GetNetwork()))
		buffer.WriteString("Kubeconfig:\n")
		buffer.WriteString(fmt.Sprintf("%s\n", cluster.GetKubeconfig()))
		buffer.WriteString("Public key:\n")
		buffer.WriteString(fmt.Sprintf("%s\n", cluster.ClusterInfo.PublicKey))
		buffer.WriteString("Private key:\n")
		buffer.WriteString(fmt.Sprintf("%s\n", cluster.ClusterInfo.PrivateKey))
		buffer.WriteString("Node Pools:\n")
		for j, nodePool := range cluster.ClusterInfo.GetNodePools() {
			if np := nodePool.GetDynamicNodePool(); np != nil {
				buffer.WriteString("----------------------------------------\n")
				buffer.WriteString(fmt.Sprintf("NodePool number: %d \n", j))
				buffer.WriteString(fmt.Sprintf("Name: %s\n", nodePool.GetName()))
				buffer.WriteString(fmt.Sprintf("Region %s\n", np.GetRegion()))
				buffer.WriteString(fmt.Sprintf("Provider specs: %v\n", np.GetProvider()))
				buffer.WriteString(fmt.Sprintf("Autoscaler conf: %v\n", np.GetAutoscalerConfig()))
				buffer.WriteString(fmt.Sprintf("Count: %d\n", np.GetCount()))

				buffer.WriteString("Nodes:\n")
				for _, node := range nodePool.GetNodes() {
					buffer.WriteString(fmt.Sprintf("Name: %s Public: %s Private: %s NodeType: %s \n", node.Name, node.GetPublic(), node.GetPrivate(), node.GetNodeType().String()))
				}
				buffer.WriteString("----------------------------------------\n")
			} else if np := nodePool.GetStaticNodePool(); np != nil {
				buffer.WriteString("----------------------------------------\n")
				buffer.WriteString(fmt.Sprintf("NodePool number: %d \n", j))
				buffer.WriteString(fmt.Sprintf("Name: %s\n", nodePool.GetName()))
				buffer.WriteString(fmt.Sprintf("IsControl: %v\n", nodePool.GetIsControl()))

				buffer.WriteString("Nodes:\n")
				for _, node := range nodePool.GetNodes() {
					buffer.WriteString(fmt.Sprintf("Name: %s Endpoint: %s Key: %s NodeType: %s \n", node.Name, node.GetPublic(), node.GetPrivate(), node.GetNodeType().String()))
				}
				buffer.WriteString("----------------------------------------\n")
			}
		}
	}
	for i, cluster := range printState.LoadBalancerClusters {
		buffer.WriteString("========================================\n")
		buffer.WriteString(fmt.Sprintf("Cluster number: %d\n", i))
		buffer.WriteString(fmt.Sprintf("Name: %s\n", cluster.ClusterInfo.GetName()))
		buffer.WriteString(fmt.Sprintf("Hash: %s\n", cluster.ClusterInfo.GetHash()))
		buffer.WriteString("Public key:\n")
		buffer.WriteString(fmt.Sprintf("%s\n", cluster.ClusterInfo.PublicKey))
		buffer.WriteString("Private key:\n")
		buffer.WriteString(fmt.Sprintf("%s\n", cluster.ClusterInfo.PrivateKey))
		buffer.WriteString("Node Pools:\n")
		for j, nodePool := range cluster.ClusterInfo.GetNodePools() {
			if np := nodePool.GetDynamicNodePool(); np != nil {
				buffer.WriteString("--------------------Dynamic Node Pool --------------------\n")
				buffer.WriteString(fmt.Sprintf("NodePool number: %d \n", j))
				buffer.WriteString(fmt.Sprintf("Name: %s\n", nodePool.GetName()))
				buffer.WriteString(fmt.Sprintf("Region %s\n", np.GetRegion()))
				buffer.WriteString(fmt.Sprintf("Provider specs: %v\n", np.GetProvider()))
				buffer.WriteString(fmt.Sprintf("Count: %d\n", np.GetCount()))

				buffer.WriteString("Nodes:\n")
				for _, node := range nodePool.GetNodes() {
					buffer.WriteString(fmt.Sprintf("Name: %s Public: %s Private: %s NodeType: %s \n", node.Name, node.GetPublic(), node.GetPrivate(), node.GetNodeType().String()))
				}
				buffer.WriteString("----------------------------------------\n")
			} else if np := nodePool.GetStaticNodePool(); np != nil {
				buffer.WriteString("------------------Static Node Pool----------------------\n")
				buffer.WriteString(fmt.Sprintf("NodePool number: %d \n", j))
				buffer.WriteString(fmt.Sprintf("Name: %s\n", nodePool.GetName()))

				buffer.WriteString("Nodes:\n")
				for _, node := range nodePool.GetNodes() {
					buffer.WriteString(fmt.Sprintf("Name: %s Endpoint: %s Key: %s NodeType: %s \n", node.Name, node.GetPublic(), node.GetPrivate(), node.GetNodeType().String()))
				}
				buffer.WriteString("----------------------------------------\n")
			}
		}
	}
	return buffer.String(), nil
}

func saveConfig(req *pb.SaveConfigRequest, saveFun saveFunction) (*pb.SaveConfigResponse, error) {
	res, err := saveFun(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to save config via %s : %w", runtime.FuncForPC(reflect.ValueOf(saveFun).Pointer()).Name() /*prints name of the function*/, err)
	}
	return res, nil
}
