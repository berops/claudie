package urls

import "os"

//Hostnames and ports on what services are listening

var (
	//TerraformerURL is a listening URL for Terraformer module
	TerraformerURL string = os.Getenv("TERRAFORMER_HOSTNAME") + ":" + os.Getenv("TERRAFORMER_PORT")
	//WireguardianURL is a listening URL for Wireguardian module
	WireguardianURL string = os.Getenv("WIREGUARDIAN_HOSTNAME") + ":" + os.Getenv("WIREGUARDIAN_PORT")
	//KubeElevenURL is a listening URL for KubeEleven module
	KubeElevenURL string = os.Getenv("KUBE_ELEVEN_HOSTNAME") + ":" + os.Getenv("KUBE_ELEVEN_PORT")
	//BuilderURL is a listening URL for Builder module
	BuilderURL string = os.Getenv("BUILDER_HOSTNAME") + ":" + os.Getenv("BUILDER_PORT")
	//ContextBoxURL is a listening URL for ContextBox module
	ContextBoxURL string = os.Getenv("CONTEXT_BOX_HOSTNAME") + ":" + os.Getenv("CONTEXT_BOX_PORT")
	//DatabaseURL is a listening URL for Database
	DatabaseURL string = os.Getenv("DATABASE_HOSTNAME") + ":" + os.Getenv("DATABASE_PORT")
)
