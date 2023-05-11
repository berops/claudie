package outboundAdapters

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/berops/claudie/internal/envs"
)

var (
	awsRegion          = envs.AwsRegion
	awsAccessKeyId     = envs.AwsAccesskeyId
	awsSecretAccessKey = envs.AwsSecretAccessKey

	dynamoURL = envs.DynamoURL
	// This DynamoDB table is used for Terraform state locking
	dynamoDBTableName = envs.DynamoTable
)

type DynamoDBAdapter struct {
	healtcheckClient *dynamodb.Client
}

func CreateDynamoDBAdapter() *DynamoDBAdapter {
	dynamoDBAdapter := &DynamoDBAdapter{
		healtcheckClient: createDynamoDBClient(),
	}

	return dynamoDBAdapter
}

// Healthcheck function checks whether
// the DynamoDB table for Terraform state locking exists or not
func (d *DynamoDBAdapter) Healthcheck() error {
	tables, err := d.healtcheckClient.ListTables(context.Background(), nil)
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

func createDynamoDBClient() *dynamodb.Client {
	return dynamodb.NewFromConfig(
		aws.Config{
			Region: awsRegion,
			Credentials: aws.CredentialsProviderFunc(
				func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{AccessKeyID: awsAccessKeyId, SecretAccessKey: awsSecretAccessKey}, nil
				},
			),

			EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: dynamoURL}, nil
				},
			),

			RetryMaxAttempts: 10,
			RetryMode:        aws.RetryModeStandard,
		},
	)
}
