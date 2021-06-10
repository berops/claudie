package main

import (
	"fmt"
	cbox "github.com/Berops/platform/services/context-box/client"
	terraformer "github.com/Berops/platform/services/terraformer/client"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

func callTerraformer(config *pb.Config) *pb.Config {
	// Create connection to Terraformer
	cc, err := grpc.Dial(ports.TerraformerPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
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

func main() {
	// If go code crash, we will get the file name and line number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Create connection to Context-box
	cc, err := grpc.Dial(ports.ContextBoxPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to Content-box server: %v", err)
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
				// TODO: Call Terrraform
				config = callTerraformer(config)
				// TODO: Call Wireguardian
				// TODO: Call KubeEleven
				err := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
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
