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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/kubernetes"
	"github.com/berops/claudie/services/terraformer/server/loadbalancer"
)

const (
	defaultTerraformerPort = 50052
)

var (
	minioEndpoint  = strings.TrimPrefix(envs.MinioURL, "http://") //minio go client does not support http/https prefix when creating handle
	minioBucket    = "claudie-tf-state-files"                     // value is hardcoded in services/terraformer/templates/backend.tpl
	minioAccessKey = envs.MinioAccessKey
	minioSecretKey = envs.MinioSecretKey
)

var (
	dynamoURL          = envs.DynamoURL
	dynamoTable        = envs.DynamoTable
	awsRegion          = envs.AwsRegion
	awsAccessKeyId     = envs.AwsAccesskeyId
	awsSecretAccessKey = envs.AwsSecretAccessKey
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
	logger := log.With().Str("project", req.ProjectName).Logger()

	clusters := []Cluster{
		kubernetes.K8Scluster{
			DesiredK8s:    req.Desired,
			CurrentK8s:    req.Current,
			ProjectName:   req.ProjectName,
			LoadBalancers: req.DesiredLbs,
		},
	}

	for _, desired := range req.DesiredLbs {
		var curr *pb.LBcluster
		for _, current := range req.CurrentLbs {
			if desired.ClusterInfo.Name == current.ClusterInfo.Name {
				curr = current
				break
			}
		}
		clusters = append(clusters, loadbalancer.LBcluster{DesiredLB: desired, CurrentLB: curr, ProjectName: req.ProjectName})
	}

	err := utils.ConcurrentExec(clusters, func(cluster Cluster) error {
		logger.Info().Str("cluster", cluster.Id()).Msgf("Creating infrastructure")

		if err := cluster.Build(); err != nil {
			return fmt.Errorf("error while building the cluster %v : %w", cluster.Id(), err)
		}
		logger.Info().Str("cluster", cluster.Id()).Msgf("Infrastructure was successfully created")
		return nil
	})
	if err != nil {
		logger.Err(err).Str("cluster", req.Desired.ClusterInfo.Name).Msgf("Error while building cluster")
		return nil, fmt.Errorf("error while building cluster %s for project %s : %w", req.Desired.ClusterInfo.Name, req.ProjectName, err)
	}

	resp := &pb.BuildInfrastructureResponse{
		Current:    req.Current,
		Desired:    req.Desired,
		CurrentLbs: req.CurrentLbs,
		DesiredLbs: req.DesiredLbs,
	}

	return resp, nil
}

func (*server) DestroyInfrastructure(ctx context.Context, req *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	var clusters []Cluster

	if req.Current != nil {
		clusters = append(clusters, kubernetes.K8Scluster{
			CurrentK8s:  req.Current,
			ProjectName: req.ProjectName,
		})
	}

	for _, lb := range req.CurrentLbs {
		clusters = append(clusters, loadbalancer.LBcluster{CurrentLB: lb, ProjectName: req.ProjectName})
	}

	mc, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: false,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to minIO: %w", err)
	}

	dynamoConnection := dynamodb.NewFromConfig(aws.Config{
		Region: awsRegion,
		Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: awsAccessKeyId, SecretAccessKey: awsSecretAccessKey}, nil
		}),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: dynamoURL}, nil
		}),
		RetryMaxAttempts: 10,
		RetryMode:        aws.RetryModeStandard,
	})

	err = utils.ConcurrentExec(clusters, func(cluster Cluster) error {
		logger := utils.CreateLoggerWithProjectAndClusterName(req.ProjectName, cluster.Id())

		logger.Info().Msgf("Destroying infrastructure")
		if err := cluster.Destroy(); err != nil {
			return fmt.Errorf("error while destroying cluster %v : %w", cluster.Id(), err)
		}

		// if it's a load-balancer there is an additional lock-file for the dns in both  MinIO and dynamoDB.
		if _, ok := cluster.(loadbalancer.LBcluster); ok {
			// Key under which the lockfile id is stored in dynamodb
			dynamoLockId, err := attributevalue.Marshal(fmt.Sprintf("%s/%s/%s-dns-md5", minioBucket, req.ProjectName, cluster.Id()))
			if err != nil {
				return fmt.Errorf("error composing state lockfile id for cluster %s: %w", cluster.Id(), err)
			}
			logger.Debug().Msgf("deleting lockfile under key: %v", dynamoLockId)
			// Remove the lockfile from dynamodb
			if _, err := dynamoConnection.DeleteItem(ctx, &dynamodb.DeleteItemInput{Key: map[string]types.AttributeValue{"LockID": dynamoLockId}, TableName: aws.String(dynamoTable)}); err != nil {
				return fmt.Errorf("failed to remove state lock file %v : %w", cluster.Id(), err)
			}

			key := fmt.Sprintf("%s/%s-dns", req.ProjectName, cluster.Id())
			if err := mc.RemoveObject(ctx, minioBucket, key, minio.RemoveObjectOptions{GovernanceBypass: true}); err != nil {
				return fmt.Errorf("failed to remove dns lock file for cluster %v: %w", cluster.Id(), err)
			}
		}
		logger.Info().Msgf("Infrastructure for cluster was successfully destroyed")

		// Key under which the lockfile id is stored in dynamodb
		dynamoLockId, err := attributevalue.Marshal(fmt.Sprintf("%s/%s/%s-md5", minioBucket, req.ProjectName, cluster.Id()))
		if err != nil {
			return fmt.Errorf("error composing state lockfile id for cluster %s: %w", cluster.Id(), err)
		}
		logger.Debug().Msgf("deleting lockfile under key: %v", dynamoLockId)
		// Remove the lockfile from dynamodb
		if _, err := dynamoConnection.DeleteItem(ctx, &dynamodb.DeleteItemInput{Key: map[string]types.AttributeValue{"LockID": dynamoLockId}, TableName: aws.String(dynamoTable)}); err != nil {
			return fmt.Errorf("failed to remove state lock file for cluster %v : %w", cluster.Id(), err)
		}

		// Key under which the state file is stored in minIO.
		key := fmt.Sprintf("%s/%s", req.ProjectName, cluster.Id())
		return mc.RemoveObject(ctx, minioBucket, key, minio.RemoveObjectOptions{
			GovernanceBypass: true,
			VersionID:        "", // currently we don't use version ID's in minIO.
		})
	})

	if err != nil {
		log.Err(err).Str("project", req.ProjectName).Msgf("Error while destroying the infrastructure for project")
		return nil, fmt.Errorf("error while destroying infrastructure for project %s : %w", req.ProjectName, err)
	}
	return &pb.DestroyInfrastructureResponse{Current: req.Current, CurrentLbs: req.CurrentLbs}, nil
}

// healthCheck function is a readiness function defined by terraformer
// it checks whether MinIO bucket exists and if dynamoDB table exists. If true, returns nil, error otherwise.
func healthCheck() error {
	// Minio bucket
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
	// dynamoDB Table
	dc := dynamodb.NewFromConfig(aws.Config{
		Region: awsRegion,
		Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: awsAccessKeyId, SecretAccessKey: awsSecretAccessKey}, nil
		}),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: dynamoURL}, nil
		}),
		RetryMaxAttempts: 10,
		RetryMode:        aws.RetryModeStandard,
	})
	tables, err := dc.ListTables(context.Background(), nil)
	if err != nil {
		return err
	}

	for _, t := range tables.TableNames {
		if t == dynamoTable {
			return nil
		}
	}
	return fmt.Errorf("dynamoDB does not contain %s table", dynamoTable)
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
	healthServer := health.NewServer()
	// Set liveness to SERVING
	healthServer.SetServingStatus("terraformer-liveness", grpc_health_v1.HealthCheckResponse_SERVING)
	// Set readiness to NOT_SERVING, as it will be changed later.
	healthServer.SetServingStatus("terraformer-readiness", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthServer)

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		// Check if terraform is in ready state every 30s
		ticker := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return nil
			case <-ticker.C:
				if err := healthCheck(); err != nil {
					healthServer.SetServingStatus("terraformer-readiness", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
				} else {
					healthServer.SetServingStatus("terraformer-readiness", grpc_health_v1.HealthCheckResponse_SERVING)
				}
			}
		}
	})

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
			err = errors.New("interrupt signal")
		}

		log.Info().Msg("Gracefully shutting down gRPC server")
		s.GracefulStop()
		healthServer.Shutdown()

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
