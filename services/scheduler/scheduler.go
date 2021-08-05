package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/urls"
	"github.com/Berops/platform/worker"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

////////////////////YAML STRUCT//////////////////////////////////////////////////

type Manifest struct {
	Name     string    `yaml:"name"`
	Clusters []Cluster `yaml:"clusters"`
}

type Cluster struct {
	Name       string     `yaml:"name"`
	Kubernetes string     `yaml:"kubernetes"`
	Network    string     `yaml:"network"`
	NodePools  []NodePool `yaml:"nodePools"`
	PrivateKey string
	PublicKey  string
}

type NodePool struct {
	Name     string   `yaml:"name"`
	Region   string   `yaml:"region"`
	Master   Master   `yaml:"master"`
	Worker   Worker   `yaml:"worker"`
	Provider Provider `yaml:"provider"`
}

type Master struct {
	Count      int32  `yaml:"count"`
	ServerType string `yaml:"server_type"`
	Image      string `yaml:"image"`
	DiskSize   uint32 `yaml:"disk_size"`
	Zone       string `yaml:"zone"`
	Location   string `yaml:"location"`
	Datacenter string `yaml:"datacenter"`
}

type Worker struct {
	Count      int32  `yaml:"count"`
	ServerType string `yaml:"server_type"`
	Image      string `yaml:"image"`
	DiskSize   uint32 `yaml:"disk_size"`
	Zone       string `yaml:"zone"`
	Location   string `yaml:"location"`
	Datacenter string `yaml:"datacenter"`
}

type Provider struct {
	Name        string `yaml:"name"`
	Credentials string `yaml:"credentials"`
}

////////////////////////////////////////////////////////////////////////////////

func MakeSSHKeyPair() (string, string) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2042)
	if err != nil {
		return "", ""
	}

	// generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return "", ""
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", ""
	}

	var pubKeyBuf strings.Builder
	pubKeyBuf.Write(ssh.MarshalAuthorizedKey(pub))

	return privKeyBuf.String(), pubKeyBuf.String()
}

func createDesiredState(config *pb.Config) (*pb.Config, error) {
	if config == nil {
		log.Println("Got nil, expected Config... \nReturning nil")
		return nil, nil
	}
	//Create yaml manifest
	d := []byte(config.GetManifest())
	err := ioutil.WriteFile("manifest.yaml", d, 0644)
	if err != nil {
		return nil, fmt.Errorf("error while creating manifest.yaml file: %v", err)
	}
	//Parse yaml to protobuf and create desiredState
	var desiredState Manifest
	yamlFile, err := ioutil.ReadFile("manifest.yaml")
	if err != nil {
		return nil, fmt.Errorf("error while reading maninfest.yaml file: %v", err)
	}
	err = yaml.Unmarshal(yamlFile, &desiredState)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling yaml file: %v", err)
	}
	//Remove yaml manifest after loading
	err = os.Remove("manifest.yaml")
	if err != nil {
		return nil, fmt.Errorf("error while removing maninfest.yaml file: %v", err)
	}

	var clusters []*pb.Cluster
	for _, cluster := range desiredState.Clusters {
		var nodePools []*pb.NodePool
		for _, nodePool := range cluster.NodePools {
			nodePools = append(nodePools, &pb.NodePool{
				Name:   nodePool.Name,
				Region: nodePool.Region,
				Master: &pb.Node{
					Count:      uint32(nodePool.Master.Count),
					ServerType: nodePool.Master.ServerType,
					Image:      nodePool.Master.Image,
					DiskSize:   nodePool.Master.DiskSize,
					Zone:       nodePool.Master.Zone,
					Location:   nodePool.Master.Location,
					Datacenter: nodePool.Master.Datacenter,
				},
				Worker: &pb.Node{
					Count:      uint32(nodePool.Worker.Count),
					ServerType: nodePool.Worker.ServerType,
					Image:      nodePool.Worker.Image,
					DiskSize:   nodePool.Worker.DiskSize,
					Zone:       nodePool.Worker.Zone,
					Location:   nodePool.Worker.Location,
					Datacenter: nodePool.Worker.Datacenter,
				},
				Provider: &pb.Provider{
					Name:        nodePool.Provider.Name,
					Credentials: nodePool.Provider.Credentials,
				},
			})
		}

		clusters = append(clusters, &pb.Cluster{
			Name:       cluster.Name,
			Kubernetes: cluster.Kubernetes,
			Network:    cluster.Network,
			PrivateKey: cluster.PrivateKey,
			PublicKey:  cluster.PublicKey,
			NodePools:  nodePools,
		})
	}

	res := &pb.Config{
		Id:       config.GetId(),
		Name:     config.GetName(),
		Manifest: config.GetManifest(),
		DesiredState: &pb.Project{
			Name:     desiredState.Name,
			Clusters: clusters,
		},
		CurrentState: config.GetCurrentState(),
		MsChecksum:   config.GetMsChecksum(),
		DsChecksum:   config.GetDsChecksum(),
		CsChecksum:   config.GetCsChecksum(),
		BuilderTTL:   config.GetBuilderTTL(),
		SchedulerTTL: config.GetSchedulerTTL(),
	}
	// Check if all clusters in a currentState have generated a SSH key pair. If not, generate a new pair for a cluster in desiredState.
KeyChecking:
	for _, clusterDesired := range res.DesiredState.Clusters {
		for _, clusterCurrent := range res.CurrentState.Clusters {
			if clusterDesired.Name == clusterCurrent.Name {
				if clusterCurrent.PublicKey != "" {
					clusterDesired.PublicKey = clusterCurrent.PublicKey
					clusterDesired.PrivateKey = clusterCurrent.PrivateKey
					continue KeyChecking
				}
			}
		}
		privateKey, publicKey := MakeSSHKeyPair()
		clusterDesired.PrivateKey = privateKey
		clusterDesired.PublicKey = publicKey
	}

	return res, nil
}

// processConfig is function used to carry out task specific to Scheduler concurrently
func processConfig(config *pb.Config, c pb.ContextBoxServiceClient) (err error) {
	config, err = createDesiredState(config)
	if err != nil {
		return
	}

	log.Println(config.GetDesiredState())
	err = cbox.SaveConfigScheduler(c, &pb.SaveConfigRequest{Config: config})
	if err != nil {
		return fmt.Errorf("error while saving the config: %v", err)
	}

	return nil
}

// healthCheck function is function used for querring readiness of the pod running this microservice
func healthCheck() error {
	res, err := createDesiredState(nil)
	if res != nil || err != nil {
		return fmt.Errorf("health check function got unexpected result")
	}
	return nil
}

func configProcessor(c pb.ContextBoxServiceClient) func() error {
	return func() error {
		res, err := cbox.GetConfigScheduler(c)
		if err != nil {
			return fmt.Errorf("error while getting config from the Scheduler: %v", err)
		}

		config := res.GetConfig()
		if config != nil {
			go processConfig(config, c)
		}

		return nil
	}
}

func main() {
	//Create connection to Context-box
	cc, err := grpc.Dial(urls.ContextBoxURL, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}

	defer cc.Close()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	// Initilize health probes
	healthChecker := healthcheck.NewClientHealthChecker("50056", healthCheck)
	healthChecker.StartProbes()

	g, ctx := errgroup.WithContext(context.Background())
	w := worker.NewWorker(10*time.Second, ctx, configProcessor(c), worker.ErrorLogger)

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

	log.Println("Stopping Scheduler: ", g.Wait())
}
