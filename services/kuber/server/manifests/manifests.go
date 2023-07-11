package manifests

import (
	_ "embed"
)

var (
	//go:embed longhorn.yaml
	LonghornManifest string

	//go:embed claudie-defaults.yaml
	ClaudieDefaultSettings string
)
