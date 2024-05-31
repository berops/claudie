package backend

import (
	_ "embed"
	"fmt"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/templateUtils"
)

//go:embed backend.tpl
var backendTemplate string

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
	// Key under which the State file will be stored.
	Key string
	// Where the backend file should be generated.
	Directory string
}

type templateData struct {
	Key         string
	BucketURL   string
	BucketName  string
	DynamoURL   string
	DynamoTable string
	Region      string
	AccessKey   string
	SecretKey   string
}

func Create(b *Backend) error {
	template := templateUtils.Templates{
		Directory: b.Directory,
	}

	tpl, err := templateUtils.LoadTemplate(backendTemplate)
	if err != nil {
		return fmt.Errorf("failed to load template file external_backend.tpl under key %q: %w", b.Key, err)
	}

	data := templateData{
		Key:         b.Key,
		BucketURL:   bucketURL,
		BucketName:  bucketName,
		DynamoURL:   dynamoURL,
		DynamoTable: dynamoTable,
		AccessKey:   awsAccessKey,
		SecretKey:   awsSecretAccessKey,
		Region:      region,
	}

	if err := template.Generate(tpl, "backend.tf", data); err != nil {
		return fmt.Errorf("failed to generate backend files under key %q : %w", b.Key, err)
	}

	return nil
}
