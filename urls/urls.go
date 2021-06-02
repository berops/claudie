package urls

import "os"

//URLs and URLs, on what services are listening

var (
	//TerraformerURL is a listening URL for Terraformer module
	TerraformerURL string = os.Getenv("TERRAFORMER_URL")
	//WireguardianURL is a listening URL for Wireguardian module
	WireguardianURL string = os.Getenv("WIREGUARDIAN_URL")
	//KubeElevenURL is a listening URL for KubeEleven module
	KubeElevenURL string = os.Getenv("KUBE_ELEVEN_URL")
	//BuilderURL is a listening URL for Builder module
	BuilderURL string = os.Getenv("BUILDER_URL")
	//ContextBoxURL is a listening URL for ContextBox module
	ContextBoxURL string = os.Getenv("CONTEXT_BOX_URL")
	//DatabaseURL is a listening URL for Database
	DatabaseURL string = os.Getenv("DATABASE_URL")
)
