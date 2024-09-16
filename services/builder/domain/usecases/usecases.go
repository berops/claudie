package usecases

import (
	"github.com/berops/claudie/services/builder/domain/ports"
	managerclient "github.com/berops/claudie/services/manager/client"
)

type Usecases struct {
	// Manager client to perform tasks related to manager
	Manager managerclient.ClientAPI
	// Terraformer connector to perform tasks related to Terraformer
	Terraformer ports.TerraformerPort
	// Ansibler connector to perform tasks related to Ansibler
	Ansibler ports.AnsiblerPort
	// KubeEleven connector to perform tasks related to KubeEleven
	KubeEleven ports.KubeElevenPort
	// Kuber connector to perform tasks related to Kuber
	Kuber ports.KuberPort
}
