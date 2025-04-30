package templates

import _ "embed"

var (
	//go:embed all-node-inventory.goini
	AllNodesInventoryTemplate string

	//go:embed uninstall-nginx.goyml
	UninstallNginx string

	//go:embed deploy-envoy.goyml
	EnvoyTemplate string

	//go:embed envoy.goyml
	EnvoyConfig string

	//go:embed envoy-dynamic-clusters.goyml
	EnvoyDynamicClusters string

	//go:embed envoy-dynamic-listeners.goyml
	EnvoyDynamicListeners string

	//go:embed envoy-docker-compose.goyml
	EnvoyDockerCompose string

	//go:embed lb-inventory.goini
	LoadbalancerInventoryTemplate string

	//go:embed node-exporter.goyml
	NodeExporterPlaybookTemplate string

	//go:embed proxy-envs.goini
	ProxyEnvsInventoryTemplate string
)
