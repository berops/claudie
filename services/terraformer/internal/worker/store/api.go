package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/healthcheck"
)

// environments variables that should be used within the implementation of [S3StateStorage]
var (
	s3Endpoint = envs.BucketEndpoint
	s3Bucket   = envs.BucketName

	awsRegion          = envs.AwsRegion
	awsAccessKeyId     = envs.AwsAccesskeyId
	awsSecretAccessKey = envs.AwsSecretAccessKey
)

var (
	// ErrKeyNotExists is returned when the key is not present in the storage implementing [S3StateStorage].
	ErrS3KeyNotExists = errors.New("key is not present in bucket")
)

const (
	// Format used to store statefiles.
	//
	// State files are always stored under <Parent>/<Child> pattern.
	KeyFormatStateFile = "%s/%s"
)

// API for communicating with S3 style state storage for managing terraform state files.
type S3StateStorage interface {
	// DeleteStateFile removes tofu state file from S3 storage.
	DeleteStateFile(ctx context.Context, key string) error

	// Stat checks whether the object exists.
	// If the key is not found the [ErrS3KeyNotExists] is returned.
	Stat(ctx context.Context, key string) error

	healthcheck.HealthChecker
}

// ObjectKey retursn the key under which a resource is stored in the S3 storage.
func ObjectKey(projectName string, subResource string) string {
	return fmt.Sprintf(KeyFormatStateFile, projectName, subResource)
}
