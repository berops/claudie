package utils

// Hostnames and ports on which services are listening
var (
	//TerraformerURL is a listening URL for Terraformer module
	TerraformerURL = GetenvOr("TERRAFORMER_HOSTNAME", "localhost") + ":" + GetenvOr("TERRAFORMER_PORT", "50052")
	//WireguardianURL is a listening URL for Wireguardian module
	WireguardianURL = GetenvOr("WIREGUARDIAN_HOSTNAME", "localhost") + ":" + GetenvOr("WIREGUARDIAN_PORT", "50053")
	//KubeElevenURL is a listening URL for KubeEleven module
	KubeElevenURL = GetenvOr("KUBE_ELEVEN_HOSTNAME", "localhost") + ":" + GetenvOr("KUBE_ELEVEN_PORT", "50054")
	//ContextBoxURL is a listening URL for ContextBox module
	ContextBoxURL = GetenvOr("CONTEXT_BOX_HOSTNAME", "localhost") + ":" + GetenvOr("CONTEXT_BOX_PORT", "50055")
	//DatabaseURL is a listening URL for Database
	DatabaseURL = GetenvOr("DATABASE_HOSTNAME", "mongodb://localhost") + ":" + GetenvOr("DATABASE_PORT", "27017")
)
