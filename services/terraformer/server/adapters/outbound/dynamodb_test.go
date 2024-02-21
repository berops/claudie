package outboundAdapters

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type DynamoDBAdapterMock struct {
	DeleteItemFunc func(ctx context.Context, input *DynamoDBDeleteItemInput) (*DynamoDBDeleteItemOutput, error)
}

func (m *DynamoDBAdapterMock) DeleteItem(ctx context.Context, input *DynamoDBDeleteItemInput) (*DynamoDBDeleteItemOutput, error) {
	return m.DeleteItemFunc(ctx, input)
}

type DynamoDBDeleteItemInput struct {
	TableName string
	Key       map[string]interface{}
}

type DynamoDBDeleteItemOutput struct {
}

func TestDeleteLockFile(t *testing.T) {
	projectName := "default-test-set-1"
	clusterId := "hetzner-test-cluster-docz0rr"
	keyFormat := "%s/%s/%s-md5"

	mockAccessKeyId := "AKIAURAGIOC5Z4CC3NX5"
	mockSecretAccessKey := "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"
	mockDynamoEndpoint := "https://dynamodb.eu-west-2.amazonaws.com"
	mockRegion := "eu-west-2"
	dynamoDBTableName = "claudie-external-test"

	// Mock DynamoDB Adapter
	dynamoDBMock := dynamodb.NewFromConfig(
		aws.Config{
			Region: mockRegion,
			Credentials: aws.CredentialsProviderFunc(
				func(ctx context.Context) (aws.Credentials, error) {
					return aws.Credentials{AccessKeyID: mockAccessKeyId, SecretAccessKey: mockSecretAccessKey}, nil
				},
			),
			EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: mockDynamoEndpoint}, nil
				},
			),
			RetryMaxAttempts: 3,
			RetryMode:        aws.RetryModeStandard,
		},
	)

	// Create DynamoDBAdapter instance with the mock
	dynamoDBAdapter := DynamoDBAdapter{Client: dynamoDBMock, healthcheckClient: dynamoDBMock}

	// Call the function
	err := dynamoDBAdapter.DeleteLockFile(context.TODO(), projectName, clusterId, keyFormat)

	// Assertions
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}
