package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/Berops/platform/healthcheck"
	kubeEleven "github.com/Berops/platform/services/kube-eleven/client"

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

// processConfig is function used to carry out task specific to Builder concurrently
func processConfig(config *pb.Config, c pb.ContextBoxServiceClient) {
	log.Println("I got config: ", config.GetName())
	config = callTerraformer(config)
	config = callWireguardian(config)
	config = callKubeEleven(config)
	config.CurrentState = config.DesiredState // Update currentState

	err := cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
	if err != nil {
		log.Fatalln("Error while saving the config", err)
	}
}

// healthCheck function is function used for querring readiness of the pod running this microservice
func healthCheck() error {
	//Check if Builder can connect to Terraformer/Wireguardian/Kube-eleven
	//Connection to these services are crucial for Builder, without them, the builder is NOT Ready
	_, err := grpc.Dial(urls.KubeElevenURL, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("could not connect to Kube-eleven: %v", err)
	}
	_, err = grpc.Dial(urls.TerraformerURL, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("could not connect to Terraformer: %v", err)
	}
	_, err = grpc.Dial(urls.WireguardianURL, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("could not connect to Wireguardian: %v", err)
	}
	return nil
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

	// Initilize health probes
	healthChecker := healthcheck.NewClientHealthChecker("50051", healthCheck)
	healthChecker.StartProbes()

	// Create limit on max goroutines
	maxGoroutines := 10
	concurrentGoroutines := make(chan struct{}, maxGoroutines) // Dummy channel to act like a semaphore
	done := make(chan bool)                                    // Channel to determine when goroutine is done

	// Fill up the dummy channel so N goroutines can start immediately
	for i := 0; i < maxGoroutines; i++ {
		concurrentGoroutines <- struct{}{}
	}

	// Monitor done channel
	// if we receive a bool, we add empty struct to signal, that new goroutine can start
	go func() {
		for {
			<-done                             // block function until done has recieved bool
			concurrentGoroutines <- struct{}{} // signal that new goroutine can start
		}
	}()

	// Main loop for getting and processing configs
	go func() {
		for {
			res, err := cbox.GetConfigBuilder(c) // Get a new config
			if err != nil {
				log.Fatalln("Error while getting config from the Builder", err)
			}

			config := res.GetConfig()
			if config != nil {
				// if we recieve a message from the channel, the goroutine can start, block otherwise
				<-concurrentGoroutines
				fmt.Println("Recieved signal to start")
				go func() {
					processConfig(config, c)
					done <- true
					fmt.Println("Done", config.GetName())
				}()
			}
			time.Sleep(5 * time.Second)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	<-ch
	fmt.Println("Stopping Builder")
}
