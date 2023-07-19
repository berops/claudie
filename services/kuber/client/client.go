package kuber

import (
	"context"
	"fmt"

	"github.com/berops/claudie/proto/pb"
)

func SetUpStorage(c pb.KuberServiceClient, req *pb.SetUpStorageRequest) (*pb.SetUpStorageResponse, error) {
	res, err := c.SetUpStorage(context.Background(), req) //sending request to the server and receiving response
	if err != nil {
		return nil, fmt.Errorf("error while calling SetUpStorage on Kuber: %w", err)
	}
	return res, nil
}

func StoreKubeconfig(c pb.KuberServiceClient, req *pb.StoreKubeconfigRequest) error {
	_, err := c.StoreKubeconfig(context.Background(), req)
	if err != nil {
		return fmt.Errorf("error while calling StoreKubeconfig on kuber: %w", err)
	}
	return nil
}

func DeleteKubeconfig(c pb.KuberServiceClient, req *pb.DeleteKubeconfigRequest) error {
	_, err := c.DeleteKubeconfig(context.Background(), req)
	if err != nil {
		return fmt.Errorf("error while calling DeleteKubeconfig on kuber: %w", err)
	}
	return nil
}

func DeleteNodes(c pb.KuberServiceClient, req *pb.DeleteNodesRequest) (*pb.DeleteNodesResponse, error) {
	res, err := c.DeleteNodes(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("error while calling DeleteNodes on kuber: %w", err)
	}
	return res, nil
}

func RemoveLbScrapeConfig(c pb.KuberServiceClient, req *pb.RemoveLBScrapeConfigRequest) error {
	_, err := c.RemoveLBScrapeConfig(context.Background(), req)
	if err != nil {
		return fmt.Errorf("error while calling RemoveLbScrapeConfig: %w", err)
	}
	return nil
}

func StoreLbScrapeConfig(c pb.KuberServiceClient, req *pb.StoreLBScrapeConfigRequest) error {
	_, err := c.StoreLBScrapeConfig(context.Background(), req)
	if err != nil {
		return fmt.Errorf("error while calling StoreLbScrapeConfig: %w", err)
	}

	return nil
}

func StoreClusterMetadata(c pb.KuberServiceClient, req *pb.StoreClusterMetadataRequest) error {
	_, err := c.StoreClusterMetadata(context.Background(), req)
	if err != nil {
		return fmt.Errorf("error while calling StoreClusterMetadata: %w", err)
	}

	return nil
}

func DeleteClusterMetadata(c pb.KuberServiceClient, req *pb.DeleteClusterMetadataRequest) error {
	_, err := c.DeleteClusterMetadata(context.Background(), req)
	if err != nil {
		return fmt.Errorf("error while calling DeleteClusterMetadata: %w", err)
	}

	return nil
}

func PatchNodes(c pb.KuberServiceClient, req *pb.PatchNodesRequest) error {
	_, err := c.PatchNodes(context.Background(), req)
	if err != nil {
		return fmt.Errorf("error while calling PatchNodes: %w", err)
	}

	return nil
}

func SetUpClusterAutoscaler(c pb.KuberServiceClient, req *pb.SetUpClusterAutoscalerRequest) error {
	_, err := c.SetUpClusterAutoscaler(context.Background(), req)
	if err != nil {
		return fmt.Errorf("error while calling SetUpClusterAutoscaler: %w", err)
	}

	return nil
}

func DestroyClusterAutoscaler(c pb.KuberServiceClient, req *pb.DestroyClusterAutoscalerRequest) error {
	_, err := c.DestroyClusterAutoscaler(context.Background(), req)
	if err != nil {
		return fmt.Errorf("error while calling DestroyClusterAutoscaler: %w", err)
	}

	return nil
}

func PatchClusterInfoConfigMap(c pb.KuberServiceClient, req *pb.PatchClusterInfoConfigMapRequest) error {
	_, err := c.PatchClusterInfoConfigMap(context.Background(), req)
	return err
}
