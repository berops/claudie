package backend

import (
	"fmt"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/templateUtils"
	"github.com/berops/claudie/services/terraformer/templates"
)

var (
	minioURL    = envs.MinioURL
	accessKey   = envs.MinioAccessKey
	secretKey   = envs.MinioSecretKey
	dynamoURL   = envs.DynamoURL
	dynamoTable = envs.DynamoTable
)

type Backend struct {
	ProjectName string
	ClusterName string
	Directory   string
}

type templateData struct {
	ProjectName string
	ClusterName string
	MinioURL    string
	AccessKey   string
	SecretKey   string
	DynamoURL   string
	DynamoTable string
}

// CreateFiles creates backend.tf file using the template from Directory.
func (b Backend) CreateFiles() error {
	template := templateUtils.Templates{Directory: b.Directory}

	tpl, err := templateUtils.LoadTemplate(templates.BackendTemplate)
	if err != nil {
		return fmt.Errorf("failed to load template file backend.tpl for %s : %w", b.ClusterName, err)
	}

	data := templateData{
		ProjectName: b.ProjectName,
		ClusterName: b.ClusterName,
		MinioURL:    minioURL,
		AccessKey:   accessKey,
		SecretKey:   secretKey,
		DynamoURL:   dynamoURL,
		DynamoTable: dynamoTable,
	}

	if err := template.Generate(tpl, "backend.tf", data); err != nil {
		return fmt.Errorf("failed to generate backend files for %s : %w", b.ClusterName, err)
	}

	return nil
}
