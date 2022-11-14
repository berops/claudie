package cbox

import (
	"context"
	"fmt"
	"reflect"
	"runtime"

	"google.golang.org/grpc"

	"github.com/Berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
)

// function to be used for saving
type saveFunction func(context.Context, *pb.SaveConfigRequest, ...grpc.CallOption) (*pb.SaveConfigResponse, error)

// SaveConfigFrontEnd calls Content-box gRPC server and saves configuration to the database
// A new config file with Id will be created if ID is empty
// if successful, returns id of the saved config, error and empty string otherwise
func SaveConfigFrontEnd(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) (string, error) {
	res, err := saveConfig(c, req, c.SaveConfigFrontEnd)
	if err != nil {
		return "", err
	} else {
		return res.Config.Id, nil
	}
}

// SaveConfigScheduler saves config from Scheduler
func SaveConfigScheduler(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	_, err := saveConfig(c, req, c.SaveConfigScheduler)
	return err
}

// SaveConfigBuilder saves config from Builder
func SaveConfigBuilder(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	_, err := saveConfig(c, req, c.SaveConfigBuilder)
	return err
}

// GetConfigScheduler gets config from queueScheduler in which are available configs for Scheduler
func GetConfigScheduler(c pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	res, err := c.GetConfigScheduler(context.Background(), &pb.GetConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("error getting scheduler config: %v", err)
	}
	return res, nil
}

// GetConfigBuilder gets config from queueBuilder in which are available configs for Builder
func GetConfigBuilder(c pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	res, err := c.GetConfigBuilder(context.Background(), &pb.GetConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("error getting builder config: %v", err)
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

// DeleteConfig sets the manifest to null so that the next invocation of the workflow
// for this config destroys the previous build infrastructure.
func DeleteConfig(c pb.ContextBoxServiceClient, id string, idType pb.IdType) error {
	res, err := c.DeleteConfig(context.Background(), &pb.DeleteConfigRequest{Id: id, Type: idType})
	if err != nil {
		return fmt.Errorf("error deleting: %v", err)
	}
	log.Info().Msgf("Config will be deleted %v", res)
	return nil
}

// DeleteConfigFromDB deletes the config from the mongoDB database.
func DeleteConfigFromDB(c pb.ContextBoxServiceClient, id string, idType pb.IdType) error {
	res, err := c.DeleteConfigFromDB(context.Background(), &pb.DeleteConfigRequest{
		Id:   id,
		Type: idType,
	})

	if err != nil {
		return fmt.Errorf("error deleting config from DB: %w", err)
	}

	log.Info().Msgf("Config was deleted from DB: %v", res)

	return nil
}

// printConfig prints a desired config with a current state info
func printConfig(c pb.ContextBoxServiceClient, id string, idType pb.IdType) (*pb.GetConfigFromDBResponse, error) {
	res, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{Id: id, Type: idType})
	if err != nil {
		log.Fatal().Msgf("Failed to get config ID %s : %v", id, err)
	}
	fmt.Println("Config name:", res.GetConfig().GetName())
	fmt.Println("Config ID:", res.GetConfig().GetId())
	fmt.Println("Project name:", res.GetConfig().GetCurrentState().GetName())
	fmt.Println("Project clusters: ")
	for i, cluster := range res.GetConfig().GetDesiredState().GetClusters() {
		fmt.Println("========================================")
		fmt.Println("Cluster number:", i)
		fmt.Println("Name:", cluster.ClusterInfo.GetName())
		fmt.Println("Hash:", cluster.ClusterInfo.GetHash())
		fmt.Println("Kubernetes version:", cluster.GetKubernetes())
		fmt.Println("Network CIDR:", cluster.GetNetwork())
		fmt.Println("Kubeconfig:")
		fmt.Println(cluster.GetKubeconfig())
		fmt.Println("Public key:")
		fmt.Println(cluster.ClusterInfo.PublicKey)
		fmt.Println("Private key:")
		fmt.Println(cluster.ClusterInfo.PrivateKey)
		fmt.Println("Node Pools:")
		for i2, nodePool := range cluster.ClusterInfo.GetNodePools() {
			fmt.Println("----------------------------------------")
			fmt.Println("NodePool number:", i2)
			fmt.Println("Name:", nodePool.GetName())
			fmt.Println("Region", nodePool.GetRegion())
			fmt.Println("Provider specs:", nodePool.GetProvider())
		}
		fmt.Println("----------------------------------------")
		fmt.Println("Cluster Nodes:")
		for _, nodePools := range cluster.ClusterInfo.GetNodePools() {
			for _, node := range nodePools.GetNodes() {
				fmt.Println("Name:", node.Name, "Public:", node.GetPublic(), "Private", node.GetPrivate(), "NodeType:", node.GetNodeType().Descriptor())
			}
		}
	}
	return res, nil
}

func saveConfig(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest, saveFun saveFunction) (*pb.SaveConfigResponse, error) {
	res, err := saveFun(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to save config via %s : %v", runtime.FuncForPC(reflect.ValueOf(saveFun).Pointer()).Name() /*prints name of the function*/, err)
	}
	log.Info().Msgf("Config %s has been saved", res.GetConfig().GetName())
	return res, nil
}
