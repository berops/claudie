package templates

import _ "embed"

var (
	//go:embed all-node-inventory.goini
	AllNodesInventoryTemplate string

	//go:embed conf.gotpl
	NginxConfigTemplate string

	//go:embed lb-inventory.goini
	LoadbalancerInventoryTemplate string

	//go:embed nginx.goyml
	NginxPlaybookTemplate string

	//go:embed node-exporter.goyml
	NodeExporterPlaybookTemplate string
)
