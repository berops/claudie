package templates

import (
	_ "embed"
)

var (
	//go:embed cluster-autoscaler.goyaml
	ClusterAutoscalerTemplate string

	//go:embed enable-ca.goyaml
	EnableClusterAutoscalerTemplate string

	//go:embed scrape-config-manifest.goyaml
	ScrapeConfigManifestTemplate string

	//go:embed scrape-config.goyaml
	ScrapeConfigTemplate string

	//go:embed storage-class.goyaml
	StorageClassTemplate string
)
