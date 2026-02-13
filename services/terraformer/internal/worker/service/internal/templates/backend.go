package templates

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
	awsAccessKey       = envs.AwsAccesskeyId
	awsSecretAccessKey = envs.AwsSecretAccessKey
	region             = envs.AwsRegion
)

type Backend struct {
	ProjectName string
	ClusterName string
	Directory   string
}

// CreateTFFile creates backend.tf file into specified Directory.
func (b Backend) CreateTFFile() error {
	template := templateUtils.Templates{Directory: b.Directory}

	tpl, err := templateUtils.LoadTemplate(backendTemplate)
	if err != nil {
		return fmt.Errorf("failed to load template file external_backend.tpl for %s : %w", b.ClusterName, err)
	}

	data := struct {
		ProjectName string
		ClusterName string
		BucketURL   string
		BucketName  string
		Region      string
		AccessKey   string
		SecretKey   string
	}{
		ProjectName: b.ProjectName,
		ClusterName: b.ClusterName,
		BucketURL:   bucketURL,
		BucketName:  bucketName,
		AccessKey:   awsAccessKey,
		SecretKey:   awsSecretAccessKey,
		Region:      region,
	}

	if err := template.Generate(tpl, "backend.tf", data); err != nil {
		return fmt.Errorf("failed to generate backend files for %s : %w", b.ClusterName, err)
	}

	return nil
}
