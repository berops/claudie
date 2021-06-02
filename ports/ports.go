package ports

import "os"

//URLs and ports on what services are listening

var (
	//TerraformerPort is a listening port for Terraformer module
	TerraformerPort string = os.Getenv("TERRAFORMER") + ":50052"
	//WireguardianPort is a listening port for Wireguardian module
	WireguardianPort string = os.Getenv("WIREGUARDIAN") + ":50053"
	//KubeElevenPort is a listening port for KubeEleven module
	KubeElevenPort string = os.Getenv("KUBE_ELEVEN") + ":50054"
	//BuilderPort is a listening port for Builder module
	BuilderPort string = os.Getenv("BUILDER") + ":50051"
	//ContextBoxPort is a listening port for ContextBox module
	ContextBoxPort string = os.Getenv("CONTEXT_BOX") + ":50055"
	//DatabasePort is a port on which database is listening
	DatabasePort string = "mongodb://" + os.Getenv("DATABASE") + ":27017"
)
