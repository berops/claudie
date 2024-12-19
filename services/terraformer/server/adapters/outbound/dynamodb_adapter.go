package outboundAdapters

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/envs"
)

var (
	// DynamoDB endpoint
	dynamoEndpoint = envs.DynamoEndpoint
	// This DynamoDB table is used for Terraform state locking
	dynamoDBTableName = envs.DynamoTable
)

type DynamoDBAdapter struct {
	Client            *dynamodb.Client
	healthcheckClient *dynamodb.Client
}

// createDynamoDBClient creates a DynamoDB client.
func createDynamoDBClient() *dynamodb.Client {
	return dynamodb.NewFromConfig(
		aws.Config{
			Region: awsRegion,
			Credentials: aws.CredentialsProviderFunc(
				func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{AccessKeyID: awsAccessKeyId, SecretAccessKey: awsSecretAccessKey}, nil
				},
			),
			RetryMaxAttempts: 10,
			RetryMode:        aws.RetryModeStandard,
		},
	)
}

// createDynamoDBClientWithEndpoint creates a DynamoDB client with a custom defined endpoint.
// It will lookup the endpoint from dynamoEndpoint variable.
// If any error occurs, then it returns the error.
func createDynamoDBClientWithEndpoint() *dynamodb.Client {
	return dynamodb.New(
		dynamodb.Options{
			Region: awsRegion,
			Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: awsAccessKeyId, SecretAccessKey: awsSecretAccessKey}, nil
			}),
			RetryMaxAttempts: 10,
			RetryMode:        aws.RetryModeStandard,
			BaseEndpoint:     aws.String(dynamoEndpoint),
		},
	)
}

// CreateDynamoDBAdapter creates 2 separate dynamoDB clients - one for healthcheck and another for general usage.
// These 2 dynamoDB clients are then used to construct a DynamoDBAdapter instance.
// Returns the DynamoDBAdapter instance.
func CreateDynamoDBAdapter() *DynamoDBAdapter {
	if dynamoEndpoint != "" {
		return &DynamoDBAdapter{
			Client:            createDynamoDBClientWithEndpoint(),
			healthcheckClient: createDynamoDBClientWithEndpoint(),
		}
	} else {
		return &DynamoDBAdapter{
			Client:            createDynamoDBClient(),
			healthcheckClient: createDynamoDBClient(),
		}
	}
}

// Healthcheck function checks whether
// the DynamoDB table for Terraform state locking exists or not.
func (d *DynamoDBAdapter) Healthcheck() error {
	tables, err := d.healthcheckClient.ListTables(context.Background(), nil)
	if err != nil {
		return err
	}

	for _, table := range tables.TableNames {
		if table == dynamoDBTableName {
			return nil
		}
	}

	return fmt.Errorf("dynamoDB does not contain %s table", dynamoDBTableName)
}

// DeleteLockFile deletes OpenTofu state lock file (related to the given cluster), from DynamoDB.
func (d *DynamoDBAdapter) DeleteLockFile(ctx context.Context, projectName, clusterId string, keyFormat string) error {
	// Get the DynamoDB key (keyname is LockID) which maps to the OpenTofu state-lock file
	key, err := attributevalue.Marshal(fmt.Sprintf(keyFormat, dynamoDBTableName, projectName, clusterId))
	if err != nil {
		return fmt.Errorf("error composing DynamoDB key for the OpenTofu state-lock file for cluster %s: %w", clusterId, err)
	}

	log.Debug().Msgf("Deleting OpenTofu state-lock file with DynamoDB key: %v", key)

	// Delete the OpenTofu state-lock file from DynamoDB
	if _, err := d.Client.DeleteItem(ctx,
		&dynamodb.DeleteItemInput{
			TableName: aws.String(dynamoDBTableName),
			Key:       map[string]types.AttributeValue{"LockID": key},
		},
	); err != nil {
		return fmt.Errorf("failed to remove OpenTofu state-lock file %v : %w", clusterId, err)
	}

	return nil
}
