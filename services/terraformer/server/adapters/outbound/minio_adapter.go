package outboundAdapters

import (
	"context"
	"fmt"
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

type MinIOAdapter struct {
	client            *minio.Client
	healthcheckClient *minio.Client
}

func createMinIOClient() (*minio.Client, error) {
	return minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: false,
	})
}

func CreateMinIOAdapter() *MinIOAdapter {
	client, err := createMinIOClient()
	if err != nil {
		log.Fatal().Msgf("Error creating client for minIO: %w", err)
	}

	healthcheckClient, err := createMinIOClient()
	if err != nil {
		log.Fatal().Msgf("Error creating healthcheck client for minIO: %w", err)
	}

	return &MinIOAdapter{
		client,
		healthcheckClient,
	}
}

func (m *MinIOAdapter) Healthcheck() error {
	bucketExists, err := m.healthcheckClient.BucketExists(context.Background(), minioBucketName)

	if !bucketExists || err != nil {
		return fmt.Errorf("error: bucket exists - %t || err: %w", bucketExists, err)
	}

	return nil
}

func (m *MinIOAdapter) DeleteTfStateFile(ctx context.Context, projectName, clusterId string, keyFormat string) error {
	key := fmt.Sprintf(keyFormat, projectName, clusterId)
	if err := m.client.RemoveObject(ctx, minioBucketName, key, minio.RemoveObjectOptions{GovernanceBypass: true}); err != nil {
		return fmt.Errorf("failed to remove dns lock file for cluster %v: %w", clusterId, err)
	}

	return nil
}
