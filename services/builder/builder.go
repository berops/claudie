package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"google.golang.org/grpc"
)

// flow permorms the sequence of gRPC calls to Terraformer, Wireguardian, KubeEleven modules (TWK)
// func flow(project *pb.Project) (*pb.Project, error) {
// 	//Terraformer
// 	project, err := messageTerraformer(project) //sending project message to Terraformer
// 	if err != nil {
// 		log.Fatalln("Error while Building Infrastructure", err)
// 	}
// 	//Wireguardian
// 	_, err = messageWireguardian(project) //sending project message to Wireguardian
// 	if err != nil {
// 		log.Fatalln("Error while creating Wireguard VPN", err)
// 	}
// 	//KubeEleven
// 	project, err = messageKubeEleven(project) //sending project message to KubeEleven
// 	if err != nil {
// 		log.Fatalln("Error while creating the cluster with KubeOne", err)
// 	}

// 	return project, nil
// }

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
			res, err := cbox.GetConfigBuilder(c)
			if err != nil {
				log.Fatalln("Error while getting config from the Builder", err)
			}
			if res.GetConfig() != nil {
				config := res.GetConfig()
				log.Println(config.GetName())
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
