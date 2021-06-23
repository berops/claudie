package main

import (
	"fmt"
	kubeEleven "github.com/Berops/platform/services/kube-eleven/client"
	"log"
	"os"
	"os/signal"
	"time"

	cbox "github.com/Berops/platform/services/context-box/client"
	terraformer "github.com/Berops/platform/services/terraformer/client"
	wireguardian "github.com/Berops/platform/services/wireguardian/client"
	"github.com/Berops/platform/urls"

	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

func callTerraformer(config *pb.Config) *pb.Config {
	// Create connection to Terraformer
	cc, err := grpc.Dial(urls.TerraformerURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to Terraformer: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	res, err := terraformer.BuildInfrastructure(c, &pb.BuildInfrastructureRequest{Config: config})
	if err != nil {
		log.Fatalln(err)
	}

	return res.GetConfig()
}

func callWireguardian(config *pb.Config) *pb.Config {
	cc, err := grpc.Dial(urls.WireguardianURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to Wireguardian: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewWireguardianServiceClient(cc)
	res, err := wireguardian.BuildVPN(c, &pb.BuildVPNRequest{Config: config})
	if err != nil {
		log.Fatalln(err)
	}
	return res.GetConfig()
}

func callKubeEleven(config *pb.Config) *pb.Config {
	cc, err := grpc.Dial(urls.KubeElevenURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to KubeEleven: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewKubeElevenServiceClient(cc)
	res, err := kubeEleven.BuildCluster(c, &pb.BuildClusterRequest{Config: config})
	if err != nil {
		log.Fatalln(err)
	}
	return res.GetConfig()
}

func main() {
	// If go code crash, we will get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Create connection to Context-box
	cc, err := grpc.Dial(urls.ContextBoxURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to Content-box: %v", err)
	}
	defer cc.Close()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)
	go func() {
		for {
			res, err := cbox.GetConfigBuilder(c) // Get a new config
			if err != nil {
				log.Fatalln("Error while getting config from the Builder", err)
			}
			if res.GetConfig() != nil {
				config := res.GetConfig()
				log.Println("I got config: ", config.GetName())
				config = callTerraformer(config)
				config = callWireguardian(config)
				config = callKubeEleven(config)
				config.CurrentState = config.DesiredState // Update currentState
				err = cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
				if err != nil {
					log.Fatalln("Error while saving the config", err)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	<-ch
	fmt.Println("Stopping Builder")
}
