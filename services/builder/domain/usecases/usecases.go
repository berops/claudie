package usecases

import "github.com/berops/claudie/services/builder/domain/ports"

type Usecases struct {
	// ContextBox connector to perform tasks related to Context-box
	ContextBox ports.ContextBoxPort
	// Terraformer connector to perform tasks related to Terraformer
	Terraformer ports.TerraformerPort
	// Ansibler connector to perform tasks related to Ansibler
	Ansibler ports.AnsiblerPort
	// KubeEleven connector to perform tasks related to KubeEleven
	KubeEleven ports.KubeElevenPort
	// Kuber connector to perform tasks related to Kuber
	Kuber ports.KuberPort
}
