package urls

import (
	"github.com/Berops/platform/utils"
)

// Hostnames and ports on which services are listening
var (
	//TerraformerURL is a listening URL for Terraformer module
	TerraformerURL = utils.GetenvOr("TERRAFORMER_HOSTNAME", "localhost") + ":" + utils.GetenvOr("TERRAFORMER_PORT", "50052")
	//WireguardianURL is a listening URL for Wireguardian module
	WireguardianURL = utils.GetenvOr("WIREGUARDIAN_HOSTNAME", "localhost") + ":" + utils.GetenvOr("WIREGUARDIAN_PORT", "50053")
	//KubeElevenURL is a listening URL for KubeEleven module
	KubeElevenURL = utils.GetenvOr("KUBE_ELEVEN_HOSTNAME", "localhost") + ":" + utils.GetenvOr("KUBE_ELEVEN_PORT", "50054")
	//ContextBoxURL is a listening URL for ContextBox module
	ContextBoxURL = utils.GetenvOr("CONTEXT_BOX_HOSTNAME", "localhost") + ":" + utils.GetenvOr("CONTEXT_BOX_PORT", "50055")
	//DatabaseURL is a listening URL for Database
	DatabaseURL = utils.GetenvOr("DATABASE_HOSTNAME", "mongodb://localhost") + ":" + utils.GetenvOr("DATABASE_PORT", "27017")
)
