package store

import (
	"context"
	"errors"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/healthcheck"
)

// environmnet variables that should be used within the implementation of [S3StateStorage] or [DynamoDB]
var (
	s3Endpoint = envs.BucketEndpoint
	s3Bucket   = envs.BucketName

	awsRegion          = envs.AwsRegion
	awsAccessKeyId     = envs.AwsAccesskeyId
	awsSecretAccessKey = envs.AwsSecretAccessKey
)

// API for communicating with DynamoDB style backends.
type DynamoDB interface {
	// DeleteLockFile removes lock file for the tofu state file from dynamoDB.
	DeleteLockFile(ctx context.Context, projectName, clusterId string, keyFormat string) error

	healthcheck.HealthChecker
}

var (
	// ErrKeyNotExists is returned when the key is not present in the storage implementing [S3StateStorage].
	ErrKeyNotExists = errors.New("key is not present in bucket")
)

// API for communicating with S3 style state storage for managing terraform state files.
type S3StateStorage interface {
	// DeleteStateFile removes tofu state file from MinIO.
	DeleteStateFile(ctx context.Context, projectName, clusterId string, keyFormat string) error
	// Stat checks whether the object exists.
	Stat(ctx context.Context, projectName, clusterId, keyFormat string) error

	healthcheck.HealthChecker
}
