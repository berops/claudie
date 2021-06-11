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

func init() {
	if BuilderURL == ":" {
		BuilderURL = "localhost:50051"
	}
	if TerraformerURL == ":" {
		TerraformerURL = "localhost:50052"
	}
	if WireguardianURL == ":" {
		WireguardianURL = "localhost:50053"
	}
	if KubeElevenURL == ":" {
		KubeElevenURL = "localhost:50054"
	}
	if ContextBoxURL == ":" {
		ContextBoxURL = "localhost:50055"
	}
	if DatabaseURL == ":" {
		DatabaseURL = "mongodb://localhost:27017"
	}
}
