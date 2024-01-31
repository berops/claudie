package backend

import (
	"fmt"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/services/terraformer/templates"
)

var (
	bucketName         = envs.BucketName
	bucketURL          = envs.BucketEndpoint
	dynamoTable        = envs.DynamoTable
	dynamoURL          = envs.DynamoEndpoint
	awsAccessKey       = envs.AwsAccesskeyId
	awsSecretAccessKey = envs.AwsSecretAccessKey
	region             = envs.AwsRegion
)

type Backend struct {
	ProjectName string
	ClusterName string
	Directory   string
}

type templateData struct {
	ProjectName string
	ClusterName string
	BucketURL   string
	BucketName  string
	DynamoURL   string
	DynamoTable string
	Region      string
	AccessKey   string
	SecretKey   string
}

// CreateTFFile creates backend.tf file into specified Directory.
func (b Backend) CreateTFFile() error {
	template := templateUtils.Templates{Directory: b.Directory}

	tpl, err := templateUtils.LoadTemplate(templates.BackendTemplate)
	if err != nil {
		return fmt.Errorf("failed to load template file external_backend.tpl for %s : %w", b.ClusterName, err)
	}
	data := templateData{
		ProjectName: b.ProjectName,
		ClusterName: b.ClusterName,
		BucketURL:   bucketURL,
		BucketName:  bucketName,
		DynamoURL:   dynamoURL,
		DynamoTable: dynamoTable,
		AccessKey:   awsAccessKey,
		SecretKey:   awsSecretAccessKey,
		Region:      region,
	}
	if err := template.Generate(tpl, "backend.tf", data); err != nil {
		return fmt.Errorf("failed to generate backend files for %s : %w", b.ClusterName, err)
	}

	return nil
}
