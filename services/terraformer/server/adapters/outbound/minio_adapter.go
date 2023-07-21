package outboundAdapters

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"

	"github.com/berops/claudie/internal/envs"
)

var (
	minioEndpoint   = strings.TrimPrefix(envs.MinioURL, "http://") //minio go client does not support http/https prefix when creating handle
	minioBucketName = "claudie-tf-state-files"                     // value is hardcoded in services/terraformer/templates/backend.tpl
	minioAccessKey  = envs.MinioAccessKey
	minioSecretKey  = envs.MinioSecretKey
)

var (
	// ErrKeyNotExists is returned when the key is not present in the object storage.
	ErrKeyNotExists = errors.New("key is not present in bucket")
)

type MinIOAdapter struct {
	client            *minio.Client
	healthcheckClient *minio.Client
}

// createMinIOClient creates and returns a MinIO client.
// If any error occurs, then it returns the error.
func createMinIOClient() (*minio.Client, error) {
	return minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: false,
	})
}

// CreateMinIOAdapter creates 2 MinIO clients first - one for healthcheck and one for general purpose.
// A MinIOAdapter instance is then constructed using those 2 clients and returned.
func CreateMinIOAdapter() *MinIOAdapter {
	client, err := createMinIOClient()
	if err != nil {
		log.Fatal().Msgf("Error creating client for minIO: %v", err)
	}

	healthcheckClient, err := createMinIOClient()
	if err != nil {
		log.Fatal().Msgf("Error creating healthcheck client for minIO: %v", err)
	}

	return &MinIOAdapter{
		client,
		healthcheckClient,
	}
}

// Healthcheck checks whether the MinIO bucket exists or not.
func (m *MinIOAdapter) Healthcheck() error {
	bucketExists, err := m.healthcheckClient.BucketExists(context.Background(), minioBucketName)

	if !bucketExists || err != nil {
		return fmt.Errorf("error: bucket exists - %t || err: %w", bucketExists, err)
	}

	return nil
}

// DeleteStateFile deletes terraform state file (related to the given cluster), from MinIO bucket.
func (m *MinIOAdapter) DeleteStateFile(ctx context.Context, projectName, clusterId string, keyFormat string) error {
	key := fmt.Sprintf(keyFormat, projectName, clusterId)
	if err := m.client.RemoveObject(ctx, minioBucketName, key, minio.RemoveObjectOptions{GovernanceBypass: true}); err != nil {
		return fmt.Errorf("failed to remove dns lock file for cluster %v: %w", clusterId, err)
	}

	return nil
}

// Stat checks whether the given object exists in storage.
func (m *MinIOAdapter) Stat(ctx context.Context, projectName, clusterId, keyFormat string) error {
	key := fmt.Sprintf(keyFormat, projectName, clusterId)
	if _, err := m.client.StatObject(ctx, minioBucketName, key, minio.StatObjectOptions{}); err != nil {
		if err, ok := err.(minio.ErrorResponse); ok {
			if err.StatusCode == http.StatusNotFound {
				return ErrKeyNotExists
			}
		}
		return fmt.Errorf("failed to check existance of object %s: %w", key, err)
	}
	return nil
}
