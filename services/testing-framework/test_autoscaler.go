package testingframework

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

const (
	scaleUpDeploymentIgnored = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment-ignore
  labels:
    app: nginx
spec:
  replicas: 4
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:1.14.2
          ports:
            - containerPort: 80
          resources:
            requests:
              memory: 8000Mi`
	scaleUpDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment-success
  labels:
    app: nginx
spec:
  replicas: 4
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:1.14.2
          ports:
            - containerPort: 80
          resources:
            requests
              memory: 500Mi`
)

func testAutoscaler(ctx context.Context, config *pb.Config) error {
	c, cc := clientConnection()
	defer func() {
		err := cc.Close()
		if err != nil {
			log.Error().Msgf("error while closing client connection : %v", err)
		}
	}()

	var clusterGroup errgroup.Group
	for _, cluster := range config.CurrentState.Clusters {
		func(cluster *pb.K8Scluster) {
			clusterGroup.Go(
				func() error {
					return applyDeployment(cluster, scaleUpDeploymentIgnored)
				})
		}(cluster)
	}

	if err := clusterGroup.Wait(); err != nil {
		return fmt.Errorf("failed to deploy scale up deployment which should be ignored: %w", err)
	}
	// Wait before checking for changes
	time.Sleep(2 * time.Minute)

	// Check if build has been started, if yes, error
	if res, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{Id: config.Id, Type: pb.IdType_HASH}); err != nil {
		if !checksumsEqual(res.Config.DsChecksum, res.Config.CsChecksum) {
			return fmt.Errorf("some cluster/s in config %s have been scaled up, when they should not", config.Name)
		}
	}
	// Apply scale up deployment
	for _, cluster := range config.CurrentState.Clusters {
		func(cluster *pb.K8Scluster) {
			clusterGroup.Go(
				func() error {
					return applyDeployment(cluster, scaleUpDeployment)
				})
		}(cluster)
	}

	if err := clusterGroup.Wait(); err != nil {
		return fmt.Errorf("failed to deploy scale up deployment : %w", err)
	}

	// Wait before checking for changes
	time.Sleep(2 * time.Minute)

	// Check if build has been started, if yes, error
	if res, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{Id: config.Id, Type: pb.IdType_HASH}); err != nil {
		if checksumsEqual(res.Config.DsChecksum, res.Config.CsChecksum) {
			return fmt.Errorf("some cluster/s in config %s have not been scaled up, when they should have", config.Name)
		}
	}

	// Wait and check if in build -> if NOT in build, error (Scale up)
	if err := configChecker(ctx, c, "autoscaling", "scale-up-test", idInfo{id: config.Id, idType: pb.IdType_HASH}); err != nil {
		return err
	}
	// Test longhorn
	if err := testLonghornDeployment(ctx, config); err != nil {
		return err
	}

	for _, cluster := range config.CurrentState.Clusters {
		func(cluster *pb.K8Scluster) {
			clusterGroup.Go(
				func() error {
					return removeDeployment(cluster, scaleUpDeployment)
				})
		}(cluster)
	}
	if err := clusterGroup.Wait(); err != nil {
		return fmt.Errorf("failed to remove scale up deployment : %w", err)
	}

	// Wait before checking for changes
	time.Sleep(12 * time.Minute)
	// Check if build has been started, if yes, error
	if res, err := c.GetConfigFromDB(context.Background(), &pb.GetConfigFromDBRequest{Id: config.Id, Type: pb.IdType_HASH}); err != nil {
		if checksumsEqual(res.Config.DsChecksum, res.Config.CsChecksum) {
			return fmt.Errorf("some cluster/s in config %s have not been scaled down, when they should have", config.Name)
		}
	}
	// Wait and check if in build -> if NOT in build, error (Scale down)
	if err := configChecker(ctx, c, "autoscaling", "scale-down-test", idInfo{id: config.Id, idType: pb.IdType_HASH}); err != nil {
		return err
	}

	// Test longhorn
	return testLonghornDeployment(ctx, config)
}

func applyDeployment(c *pb.K8Scluster, deployment string) error {
	kc := kubectl.Kubectl{Kubeconfig: c.Kubeconfig, MaxKubectlRetries: 5}
	if err := kc.KubectlApplyString(deployment, ""); err != nil {
		return fmt.Errorf("failed to apply deployment on cluster %s : %w", c.ClusterInfo.Name, err)
	}
	return nil
}

func removeDeployment(c *pb.K8Scluster, deployment string) error {
	kc := kubectl.Kubectl{Kubeconfig: c.Kubeconfig, MaxKubectlRetries: 5}
	if err := kc.KubectlDeleteString(deployment, ""); err != nil {
		return fmt.Errorf("failed to remove deployment on cluster %s : %w", c.ClusterInfo.Name, err)
	}
	return nil
}

// clientConnection will return new client connection to Context-box
func clientConnection() (pb.ContextBoxServiceClient, *grpc.ClientConn) {
	cc, err := utils.GrpcDialWithInsecure("context-box", envs.ContextBoxURL)
	if err != nil {
		log.Fatal().Msgf("Failed to create client connection to context-box : %v", err)
	}

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	return c, cc
}

// checksumsEq will check if two checksums are equal
func checksumsEqual(checksum1 []byte, checksum2 []byte) bool {
	if len(checksum1) > 0 && len(checksum2) > 0 && bytes.Equal(checksum1, checksum2) {
		return true
	} else {
		return false
	}
}
