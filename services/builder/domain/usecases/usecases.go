package usecases

import "github.com/berops/claudie/services/builder/domain/ports"

type Usecases struct {
	ContextBox  ports.ContextBoxPort
	Terraformer ports.TerraformerPort
	Ansibler    ports.AnsiblerPort
	KubeEleven  ports.KubeElevenPort
	Kuber       ports.KuberPort
}
