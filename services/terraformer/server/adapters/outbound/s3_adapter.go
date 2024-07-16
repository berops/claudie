package outboundAdapters

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/berops/claudie/internal/envs"
)

var (
	s3Endpoint = envs.BucketEndpoint
	s3Bucket   = envs.BucketName
)

type S3Adapter struct {
	client            *s3.Client
	healthcheckClient *s3.Client
}

// createS3Client creates and returns a S3 client.
// If any error occurs, then it returns the error.
func createS3Client() *s3.Client {
	return s3.NewFromConfig(
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

// createS3ClientWithEndpoint creates and returns a S3 client, with custom endpoint.
// It will lookup the endpoint from s3Endpoint variable.
// If any error occurs, then it returns the error.
func createS3ClientWithEndpoint() *s3.Client {
	return s3.New(
		s3.Options{
			Region: awsRegion,
			Credentials: aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{AccessKeyID: awsAccessKeyId, SecretAccessKey: awsSecretAccessKey}, nil
			}),
			RetryMaxAttempts:   10,
			RetryMode:          aws.RetryModeStandard,
			EndpointResolverV2: &immutableResolver{s3Endpoint},
		},
	)
}

// CreateS3Adapter creates 2 s3 clients first - one for healthcheck and one for general purpose.
// A S3Adapter instance is then constructed using those 2 clients and returned.
// S3Adapter implements StateStoragePort interface
func CreateS3Adapter() *S3Adapter {
	if s3Endpoint != "" {
		return &S3Adapter{
			client:            createS3ClientWithEndpoint(),
			healthcheckClient: createS3ClientWithEndpoint(),
		}
	} else {
		return &S3Adapter{
			client:            createS3Client(),
			healthcheckClient: createS3Client(),
		}
	}
}

// Healthcheck checks whether the S3 bucket exists or not.
func (s *S3Adapter) Healthcheck() error {
	_, err := s.healthcheckClient.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: aws.String(s3Bucket),
	})

	if err != nil {
		return fmt.Errorf("error creating healthcheck client for AWS S3: %w", err)
	}
	return nil
}

// DeleteStateFile deletes terraform state file (related to the given cluster), from S3 bucket.
func (s *S3Adapter) DeleteStateFile(ctx context.Context, projectName, clusterId string, keyFormat string) error {
	key := fmt.Sprintf(keyFormat, projectName, clusterId)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s3Bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("failed to remove dns lock file for cluster %v: %w", clusterId, err)
	}

	return nil
}

// Stat checks whether the given object exists in storage.
func (s *S3Adapter) Stat(ctx context.Context, projectName, clusterId, keyFormat string) error {
	key := fmt.Sprintf(keyFormat, projectName, clusterId)
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s3Bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("failed to check existence of object %s: %w", key, err)
	}
	return nil
}
