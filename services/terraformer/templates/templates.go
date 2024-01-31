package templates

import "embed"

var (
	//go:embed providers.tpl
	ProvidersTemplate string
	//go:embed backend.tpl
	BackendTemplate string
	//go:embed *
	CloudProviderTemplates embed.FS
)
