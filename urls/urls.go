package urls

import "os"

// Hostnames and ports on what services are listening
var (
	//TerraformerURL is a listening URL for Terraformer module
	TerraformerURL = os.Getenv("TERRAFORMER_HOSTNAME") + ":" + os.Getenv("TERRAFORMER_PORT")
	//WireguardianURL is a listening URL for Wireguardian module
	WireguardianURL = os.Getenv("WIREGUARDIAN_HOSTNAME") + ":" + os.Getenv("WIREGUARDIAN_PORT")
	//KubeElevenURL is a listening URL for KubeEleven module
	KubeElevenURL = os.Getenv("KUBE_ELEVEN_HOSTNAME") + ":" + os.Getenv("KUBE_ELEVEN_PORT")
	//ContextBoxURL is a listening URL for ContextBox module
	ContextBoxURL = os.Getenv("CONTEXT_BOX_HOSTNAME") + ":" + os.Getenv("CONTEXT_BOX_PORT")
	//DatabaseURL is a listening URL for Database
	DatabaseURL = os.Getenv("DATABASE_HOSTNAME") + ":" + os.Getenv("DATABASE_PORT")
	//KuberURL is a listening URL for Kuber module
	KuberURL = os.Getenv("KUBER_HOSTNAME") + ":" + os.Getenv("KUBER_PORT")
	//MinioURL is a listening URL for Minio deployment
	MinioURL = os.Getenv("MINIO_HOSTNAME") + ":" + os.Getenv("MINIO_PORT")
)

func init() {
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
	if KuberURL == ":" {
		KuberURL = "localhost:50057"
	}
	if MinioURL == ":" {
		MinioURL = "http://localhost:9000"
	}
}
