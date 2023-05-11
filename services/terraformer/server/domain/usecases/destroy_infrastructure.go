package usecases

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
	"github.com/berops/claudie/services/terraformer/server/kubernetes"
	"github.com/berops/claudie/services/terraformer/server/loadbalancer"
)

var (
	awsRegion          = envs.AwsRegion
	awsAccessKeyId     = envs.AwsAccesskeyId
	awsSecretAccessKey = envs.AwsSecretAccessKey

	dynamoURL = envs.DynamoURL
	// This DynamoDB table is used for Terraform state locking
	dynamoDBTableName = envs.DynamoTable
)

var (
	minioEndpoint   = strings.TrimPrefix(envs.MinioURL, "http://") //minio go client does not support http/https prefix when creating handle
	minioBucketName = "claudie-tf-state-files"                     // value is hardcoded in services/terraformer/templates/backend.tpl
	minioAccessKey  = envs.MinioAccessKey
	minioSecretKey  = envs.MinioSecretKey
)

func (u *Usecases) DestroyInfrastructure(ctx context.Context, request *pb.DestroyInfrastructureRequest) (*pb.DestroyInfrastructureResponse, error) {
	var clusters []Cluster

	if request.Current != nil {
		clusters = append(clusters, kubernetes.K8Scluster{
			CurrentK8s:  request.Current,
			ProjectName: request.ProjectName,
		})
	}

	for _, lb := range request.CurrentLbs {
		clusters = append(clusters, loadbalancer.LBcluster{CurrentLB: lb, ProjectName: request.ProjectName})
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
		log.Info().Msgf("Destroying infrastructure for cluster %s project %s", cluster.Id(), request.ProjectName)
		if err := cluster.Destroy(); err != nil {
			return fmt.Errorf("error while destroying cluster %v : %w", cluster.Id(), err)
		}

		// if it's a load-balancer there is an additional lock-file for the dns in both  MinIO and dynamoDB.
		if _, ok := cluster.(loadbalancer.LBcluster); ok {
			// Key under which the lockfile id is stored in dynamodb
			dynamoLockId, err := attributevalue.Marshal(fmt.Sprintf("%s/%s/%s-dns-md5", minioBucketName, request.ProjectName, cluster.Id()))
			if err != nil {
				return fmt.Errorf("error composing state lockfile id for cluster %s: %w", cluster.Id(), err)
			}
			log.Debug().Msgf("deleting lockfile under key: %v", dynamoLockId)
			// Remove the lockfile from dynamodb
			if _, err := dynamoConnection.DeleteItem(ctx, &dynamodb.DeleteItemInput{Key: map[string]types.AttributeValue{"LockID": dynamoLockId}, TableName: aws.String(dynamoDBTableName)}); err != nil {
				return fmt.Errorf("failed to remove state lock file %v : %w", cluster.Id(), err)
			}

			key := fmt.Sprintf("%s/%s-dns", request.ProjectName, cluster.Id())
			if err := mc.RemoveObject(ctx, minioBucketName, key, minio.RemoveObjectOptions{GovernanceBypass: true}); err != nil {
				return fmt.Errorf("failed to remove dns lock file for cluster %v: %w", cluster.Id(), err)
			}
		}
		log.Info().Msgf("Infrastructure for cluster %s project %s was successfully destroyed", cluster.Id(), request.ProjectName)

		// Key under which the lockfile id is stored in dynamodb
		dynamoLockId, err := attributevalue.Marshal(fmt.Sprintf("%s/%s/%s-md5", minioBucketName, request.ProjectName, cluster.Id()))
		if err != nil {
			return fmt.Errorf("error composing state lockfile id for cluster %s: %w", cluster.Id(), err)
		}
		log.Debug().Msgf("deleting lockfile under key: %v", dynamoLockId)
		// Remove the lockfile from dynamodb
		if _, err := dynamoConnection.DeleteItem(ctx, &dynamodb.DeleteItemInput{Key: map[string]types.AttributeValue{"LockID": dynamoLockId}, TableName: aws.String(dynamoDBTableName)}); err != nil {
			return fmt.Errorf("failed to remove state lock file for cluster %v : %w", cluster.Id(), err)
		}

		// Key under which the state file is stored in minIO.
		key := fmt.Sprintf("%s/%s", request.ProjectName, cluster.Id())
		return mc.RemoveObject(ctx, minioBucketName, key, minio.RemoveObjectOptions{
			GovernanceBypass: true,
			VersionID:        "", // currently we don't use version ID's in minIO.
		})
	})

	if err != nil {
		log.Error().Msgf("Error while destroying the infrastructure for project %s : %s", request.ProjectName, err)
		return nil, fmt.Errorf("error while destroying infrastructure for project %s : %w", request.ProjectName, err)
	}
	return &pb.DestroyInfrastructureResponse{Current: request.Current, CurrentLbs: request.CurrentLbs}, nil
}
