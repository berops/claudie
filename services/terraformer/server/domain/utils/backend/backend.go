package backend

import (
	"fmt"

	"github.com/berops/claudie/internal/envs"
	"github.com/berops/claudie/internal/templateUtils"
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

// CreateTFFile creates backend.tf file into specified Directory.
func (b Backend) CreateTFFile() error {
	template := templateUtils.Templates{Directory: b.Directory}
	templateLoader := templateUtils.TemplateLoader{Directory: templateUtils.TerraformerTemplates}

	tpl, err := templateLoader.LoadTemplate("backend.tpl")
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
