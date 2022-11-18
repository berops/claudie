package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Berops/claudie/internal/envs"
	"github.com/Berops/claudie/internal/healthcheck"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
	"github.com/Berops/claudie/services/terraformer/server/kubernetes"
	"github.com/Berops/claudie/services/terraformer/server/loadbalancer"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"	
	"github.com/rs/zerolog/log"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	defaultTerraformerPort = 50052
)

var (
	minioEndpoint  = strings.TrimPrefix(envs.MinioURL, "http://") //minio go client does not support http/https prefix when creating handle
	minioBucket    = "claudie-tf-state-files"                     // value is hardcoded in services/terraformer/templates/backend.tpl
	minioAccessKey = envs.MinioAccessKey
	minioSecretKey = envs.MinioSecretKey
	dynamoURL      = envs.DynamoURL
	dynamoTable    = envs.DynamoTable
)

type server struct {
	pb.UnimplementedTerraformerServiceServer
}

type Cluster interface {
	Build() error
	Destroy() error
	Id() string
}

func (*server) BuildInfrastructure(ctx context.Context, req *pb.BuildInfrastructureRequest) (*pb.BuildInfrastructureResponse, error) {
	currentState := req.GetCurrentState()
	desiredState := req.GetDesiredState()
	projectName := desiredState.Name
	clusterToLBs := make(map[string][]*pb.LBcluster)

	var errGroup errgroup.Group
	var clusters []Cluster

	// Get LB clusters
	for _, desiredLB := range desiredState.LoadBalancerClusters {
		clusterToLBs[desiredLB.TargetedK8S] = append(clusterToLBs[desiredLB.TargetedK8S], desiredLB)

		var existingCluster *pb.LBcluster
		for _, currentLB := range currentState.LoadBalancerClusters {
			if desiredLB.ClusterInfo.Name == currentLB.ClusterInfo.Name {
				existingCluster = currentLB
				break
			}
		}

		clusters = append(clusters, loadbalancer.LBcluster{DesiredLB: desiredLB, CurrentLB: existingCluster, ProjectName: projectName})
	}

	// Get kubernetes clusters
	for _, desiredK8s := range desiredState.Clusters {
		var existingCluster *pb.K8Scluster
		for _, currentK8s := range currentState.Clusters {
			if desiredK8s.ClusterInfo.Name == currentK8s.ClusterInfo.Name {
				existingCluster = currentK8s
				break
			}
		}

		clusters = append(clusters, kubernetes.K8Scluster{
			DesiredK8s:    desiredK8s,
			CurrentK8s:    existingCluster,
			ProjectName:   projectName,
			LoadBalancers: clusterToLBs[desiredK8s.ClusterInfo.Name],
		})
	}

	// Build clusters concurrently
	for _, cluster := range clusters {
		func(c Cluster) {
			errGroup.Go(func() error {
				if err := c.Build(); err != nil {
					return fmt.Errorf("error while building the cluster %v : %w", cluster.Id(), err)
				}
				return nil
			})
		}(cluster)
	}
	if err := errGroup.Wait(); err != nil {
		log.Error().Msgf("Failed to build infra for project %s : %s", projectName, err.Error())
		return &pb.BuildInfrastructureResponse{
				CurrentState: currentState,
				DesiredState: desiredState,
				ErrorMessage: fmt.Sprintf("BuildInfrastructure got error: %s", err.Error())},
			fmt.Errorf("BuildInfrastructure got error: %s", err.Error())
	}
	log.Info().Msgf("Infrastructure was successfully generated for project %s", projectName)
	return &pb.BuildInfrastructureResponse{
		CurrentState: currentState,
		DesiredState: desiredState,
		ErrorMessage: "",
	}, nil
}

func (*server) DestroyInfrastructure(ctx context.Context, req *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	config := req.GetConfig()
	projectName := config.CurrentState.Name

	// Get kubernetes clusters
	var clusters []Cluster
	for _, k8s := range config.CurrentState.Clusters {
		clusters = append(clusters, kubernetes.K8Scluster{CurrentK8s: k8s, ProjectName: projectName})
	}

	// Get LB clusters
	for _, lb := range config.CurrentState.LoadBalancerClusters {
		clusters = append(clusters, loadbalancer.LBcluster{CurrentLB: lb, ProjectName: projectName})
	}

	mc, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: false,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to minIO: %w", err)
	}

	dynamoConfig, err := awsConfig.LoadDefaultConfig(ctx,
		awsConfig.WithEndpointResolver(aws.EndpointResolverFunc(
			func(service, region string) (aws.Endpoint, error) {
				return aws.Endpoint{URL: dynamoURL}, nil
			})),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to dynamodb: %w", err)
	}

	dynamoConnection := dynamodb.NewFromConfig(dynamoConfig)
	
	// Destroy clusters concurrently
	var errGroup errgroup.Group
	for _, cluster := range clusters {
		func(c Cluster) {
			errGroup.Go(func() error {
				if err := c.Destroy(); err != nil {
					return fmt.Errorf("failed to destroy cluster %v : %w", cluster.Id(), err)
				}

				// Key under which the state file is stored in minIO.
				key := fmt.Sprintf("%s/%s", projectName, c.Id())

				// Type assertion to retrive current cluster Name and Hash
				currentCluster, ok := cluster.(kubernetes.K8Scluster)
				if !ok {
					log.Warn().Msgf("Failed to convert to type K8Scluster %s", config.Name)
				} else {

					// Generate dynamodb lockfile id
					dynamoLockId, err := attributevalue.Marshal(
						fmt.Sprintf("%s/%s/%s-%s-md5",
							minioBucket, projectName,
							currentCluster.CurrentK8s.ClusterInfo.Name,
							currentCluster.CurrentK8s.ClusterInfo.Hash),
					)

					if err != nil {
						log.Warn().Msgf("Error composing state lockfile id for cluster %s : %s", c.Id(), config.Name)
					}

					// Remove the lockfile from dynamodb
					_, err = dynamoConnection.DeleteItem(ctx, &dynamodb.DeleteItemInput{
						Key: map[string]types.AttributeValue{"LockID": dynamoLockId}, TableName: aws.String(dynamoTable),
					})
					
					if err != nil {
						return fmt.Errorf("failed to remove state lock file %v : %w", cluster.Id(), err)
					}
				}

				return mc.RemoveObject(ctx, minioBucket, key, minio.RemoveObjectOptions{
					GovernanceBypass: true,
					VersionID:        "", // currently we don't use version ID's in minIO.
				})
			})
		}(cluster)
	}

	if err := errGroup.Wait(); err != nil {
		config.ErrorMessage = err.Error()
		log.Error().Msgf("Failed to destroy the infra for project %s : %s", config.Name, err.Error())
		return &pb.DestroyInfrastructureResponse{Config: config}, fmt.Errorf("failed to destroy infrastructure: %s", err.Error())
	}

	log.Info().Msgf("Infra for project %s was successfully destroyed", config.Name)
	config.ErrorMessage = ""
	return &pb.DestroyInfrastructureResponse{Config: config}, nil
}

// healthCheck function is a readiness function defined by terraformer
// it checks whether bucket exists. If true, returns nil, error otherwise
func healthCheck() error {
	mc, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return err
	}

	exists, err := mc.BucketExists(context.Background(), minioBucket)
	if !exists || err != nil {
		return fmt.Errorf("error: bucket exists %t || err: %w", exists, err)
	}
	return nil
}

func main() {
	// initialize logger
	utils.InitLog("terraformer")

	// Set the context-box port
	terraformerPort := utils.GetenvOr("TERRAFORMER_PORT", fmt.Sprint(defaultTerraformerPort))

	// Start Terraformer Service
	trfAddr := net.JoinHostPort("0.0.0.0", terraformerPort)
	lis, err := net.Listen("tcp", trfAddr)
	if err != nil {
		log.Fatal().Msgf("Failed to listen on %v", err)
	}
	log.Info().Msgf("Terraformer service is listening on: %s", trfAddr)

	s := grpc.NewServer()
	pb.RegisterTerraformerServiceServer(s, &server{})

	// Add health service to gRPC
	// Here we pass our custom readiness probe
	healthService := healthcheck.NewServerHealthChecker(terraformerPort, "TERRAFORMER_PORT", healthCheck)
	grpc_health_v1.RegisterHealthServer(s, healthService)

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(ch)

		// wait for either the received signal or
		// check if an error occurred in other
		// go-routines.
		var err error
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case sig := <-ch:
			log.Info().Msgf("Received signal %v", sig)
			err = errors.New("terraformer interrupt signal")
		}

		log.Info().Msg("Gracefully shutting down gRPC server")
		s.GracefulStop()

		// Sometimes when the container terminates gRPC logs the following message:
		// rpc error: code = Unknown desc = Error: No such container: hash of the container...
		// It does not affect anything as everything will get terminated gracefully
		// this time.Sleep fixes it so that the message won't be logged.
		time.Sleep(1 * time.Second)

		return err
	})

	g.Go(func() error {
		if err := s.Serve(lis); err != nil {
			return fmt.Errorf("terraformer failed to serve: %w", err)
		}
		log.Info().Msg("Finished listening for incoming connections")
		return nil
	})

	log.Info().Msgf("Stopping Terraformer: %v", g.Wait())
}
