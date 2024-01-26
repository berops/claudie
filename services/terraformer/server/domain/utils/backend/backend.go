package backend

import (
	"fmt"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/services/terraformer/templates"
)

var (
	minioURL           = envs.MinioURL
	minioAccessKey     = envs.MinioAccessKey
	minioSecretKey     = envs.MinioSecretKey
	dynamoURL          = envs.DynamoURL
	dynamoTable        = envs.DynamoTable
	awsAccessKey       = envs.AwsAccesskeyId
	awsSecretAccessKey = envs.AwsSecretAccessKey
	region             = envs.AwsRegion
	externalBucketName = envs.ExternalS3Bucket
)

type Backend struct {
	ProjectName string
	ClusterName string
	Directory   string
}

type templateData struct {
	ProjectName    string
	ClusterName    string
	MinioURL       string
	MinioAccessKey string
	MinioSecretKey string
	DynamoURL      string
	DynamoTable    string
	S3Name         string
	Region         string
	AwsAccessKey   string
	AwsSecretKey   string
}

// CreateTFFile creates backend.tf file into specified Directory.
func (b Backend) CreateTFFile() error {
	template := templateUtils.Templates{Directory: b.Directory}

	// If the EXTERNAL_S3_BUCKET env is used, use the external_backend.tpl file
	if externalBucketName != "" {
		tpl, err := templateUtils.LoadTemplate(templates.ExternalBackendTemplate)
		if err != nil {
			return fmt.Errorf("failed to load template file external_backend.tpl for %s : %w", b.ClusterName, err)
		}
		data := templateData{
			ProjectName:  b.ProjectName,
			ClusterName:  b.ClusterName,
			S3Name:       externalBucketName,
			Region:       region,
			AwsAccessKey: awsAccessKey,
			AwsSecretKey: awsSecretAccessKey,
			DynamoTable:  dynamoTable,
		}
		if err := template.Generate(tpl, "backend.tf", data); err != nil {
			return fmt.Errorf("failed to generate backend files for %s : %w", b.ClusterName, err)
		}
	} else {
		tpl, err := templateUtils.LoadTemplate(templates.BackendTemplate)
		if err != nil {
			return fmt.Errorf("failed to load template file backend.tpl for %s : %w", b.ClusterName, err)
		}

		data := templateData{
			ProjectName:    b.ProjectName,
			ClusterName:    b.ClusterName,
			MinioURL:       minioURL,
			MinioAccessKey: minioAccessKey,
			MinioSecretKey: minioSecretKey,
			DynamoURL:      dynamoURL,
			DynamoTable:    dynamoTable,
		}

		if err := template.Generate(tpl, "backend.tf", data); err != nil {
			return fmt.Errorf("failed to generate backend files for %s : %w", b.ClusterName, err)
		}
	}
	return nil
}
