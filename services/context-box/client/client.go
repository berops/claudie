package cbox

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/Berops/platform/proto/pb"
	"github.com/rs/zerolog/log"
)

// SaveConfigFrontEnd calls Content-box gRPC server and saves configuration to the mongoDB database
// A new config file with Id will be created if ID is empty
// returns id of the saved config and error
func SaveConfigFrontEnd(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) (string, error) {
	res, err := saveConfig("frontend", c, req, c.SaveConfigFrontEnd)
	if err != nil {
		return "", err
	} else {
		return res.Config.Id, nil
	}
}

// SaveConfigScheduler saves config from Scheduler
func SaveConfigScheduler(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	_, err := saveConfig("builder", c, req, c.SaveConfigScheduler)
	return err
}

// SaveConfigBuilder saves config from Scheduler
func SaveConfigBuilder(c pb.ContextBoxServiceClient, req *pb.SaveConfigRequest) error {
	_, err := saveConfig("builder", c, req, c.SaveConfigBuilder)
	return err
}

func saveConfig(
	service string,
	c pb.ContextBoxServiceClient,
	req *pb.SaveConfigRequest,
	saveFn func(context.Context, *pb.SaveConfigRequest, ...grpc.CallOption) (*pb.SaveConfigResponse, error)) (*pb.SaveConfigResponse, error) {
	log.Info().Msgf("Saving %s config", service)

	res, err := saveFn(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("Failed to save %s config: %v", service, err)
	}

	log.Info().Msgf("Config %s has been saved", res.GetConfig().GetName())
	return res, nil
}

// GetConfigScheduler gets config from queueScheduler in which are available configs for Scheduler
func GetConfigScheduler(c pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	res, err := c.GetConfigScheduler(context.Background(), &pb.GetConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("Error getting scheduler config: %v", err)
	}

	return res, nil
}

// GetConfigBuilder gets config from queueBuilder in which are available configs for Builder
func GetConfigBuilder(c pb.ContextBoxServiceClient) (*pb.GetConfigResponse, error) {
	res, err := c.GetConfigBuilder(context.Background(), &pb.GetConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("Error getting builder config: %v", err)
	}

	return res, nil
}

// GetAllConfigs calls Content-box gRPC server and returns all configs from the mongoDB database
func GetAllConfigs(c pb.ContextBoxServiceClient) (*pb.GetAllConfigsResponse, error) {
	res, err := c.GetAllConfigs(context.Background(), &pb.GetAllConfigsRequest{})
	if err != nil {
		return nil, fmt.Errorf("Unexpected error: %v", err)
	}

	return res, nil
}

// DeleteConfig deletes object from the mongoDB database with a specified Id
func DeleteConfig(c pb.ContextBoxServiceClient, id string, idType pb.IdType) error {
	res, err := c.DeleteConfig(context.Background(), &pb.DeleteConfigRequest{Id: id, Type: idType})
	if err != nil {
		return fmt.Errorf("Error deleting: %v", err)
	}

	log.Info().Msgf("Config was deleted %v", res)
	return nil
}

// PrintConfig prints a desired config with a current state
func PrintConfig(c pb.ContextBoxServiceClient, id string, idType pb.IdType) (*pb.GetConfigFromDBResponse, error) {
	res, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{Id: id, Type: idType})
	if err != nil {
		log.Fatal().Msgf("Failed to get config ID %s : %v", id, err)
	}
	fmt.Println("Config name:", res.GetConfig().GetName())
	fmt.Println("Config ID:", res.GetConfig().GetId())
	fmt.Println("Project name:", res.GetConfig().GetCurrentState().GetName())
	fmt.Println("Project clusters: ")
	for i, cluster := range res.GetConfig().GetCurrentState().GetClusters() {
		fmt.Println("========================================")
		fmt.Println("Cluster number:", i)
		fmt.Println("Name:", cluster.ClusterInfo.GetName())
		fmt.Println("Kubernetes version:", cluster.GetKubernetes())
		fmt.Println("Network CIDR:", cluster.GetNetwork())
		fmt.Println("Kubeconfig:")
		fmt.Println(cluster.GetKubeconfig())
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
	//fmt.Println(res.GetConfig().GetCurrentState().GetClusters())
	return res, nil
}
