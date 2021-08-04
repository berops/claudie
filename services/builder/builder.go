package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/Berops/platform/healthcheck"
	kubeEleven "github.com/Berops/platform/services/kube-eleven/client"
	"github.com/Berops/platform/worker"
	"golang.org/x/sync/errgroup"

	cbox "github.com/Berops/platform/services/context-box/client"
	terraformer "github.com/Berops/platform/services/terraformer/client"
	wireguardian "github.com/Berops/platform/services/wireguardian/client"
	"github.com/Berops/platform/urls"

	"github.com/Berops/platform/proto/pb"
	"google.golang.org/grpc"
)

func callTerraformer(config *pb.Config) (*pb.Config, error) {
	// Create connection to Terraformer
	cc, err := grpc.Dial(urls.TerraformerURL, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not connect to Terraformer: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewTerraformerServiceClient(cc)
	res, err := terraformer.BuildInfrastructure(c, &pb.BuildInfrastructureRequest{Config: config})
	if err != nil {
		return nil, err
	}

	return res.GetConfig(), nil
}

func callWireguardian(config *pb.Config) (*pb.Config, error) {
	cc, err := grpc.Dial(urls.WireguardianURL, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not connect to Wireguardian: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewWireguardianServiceClient(cc)
	res, err := wireguardian.BuildVPN(c, &pb.BuildVPNRequest{Config: config})
	if err != nil {
		return nil, err
	}

	return res.GetConfig(), nil
}

func callKubeEleven(config *pb.Config) (*pb.Config, error) {
	cc, err := grpc.Dial(urls.KubeElevenURL, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not connect to KubeEleven: %v", err)
	}
	defer cc.Close()
	// Creating the client
	c := pb.NewKubeElevenServiceClient(cc)
	res, err := kubeEleven.BuildCluster(c, &pb.BuildClusterRequest{Config: config})
	if err != nil {
		return nil, err
	}

	return res.GetConfig(), nil
}

// processConfig is function used to carry out task specific to Builder concurrently
func processConfig(config *pb.Config, c pb.ContextBoxServiceClient) (err error) {
	log.Println("I got config: ", config.GetName())

	config, err = callTerraformer(config)
	if err != nil {
		return
	}

	config, err = callWireguardian(config)
	if err != nil {
		return
	}

	config, err = callKubeEleven(config)
	if err != nil {
		return
	}

	config.CurrentState = config.DesiredState // Update currentState

	err = cbox.SaveConfigBuilder(c, &pb.SaveConfigRequest{Config: config})
	if err != nil {
		return fmt.Errorf("error while saving the config: %v", err)
	}

	return nil
}

func configProcessor(c pb.ContextBoxServiceClient) func() error {
	return func() error {
		res, err := cbox.GetConfigBuilder(c) // Get a new config
		if err != nil {
			return fmt.Errorf("Error while getting config from the Builder: %v", err)
		}

		config := res.GetConfig()
		if config != nil {
			go processConfig(config, c)
		}

		return nil
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

	g, ctx := errgroup.WithContext(context.Background())
	w := worker.NewWorker(5*time.Second, ctx, configProcessor(c), worker.ErrorLogger)

	{
		g.Go(func() error {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt)
			defer signal.Stop(ch)
			<-ch
			return errors.New("interrupt signal")
		})
	}
	{
		g.Go(func() error {
			w.Run()
			return nil
		})
	}

	log.Println("Stopping Builder: ", g.Wait())
}
