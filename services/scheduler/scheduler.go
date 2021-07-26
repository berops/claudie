package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
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
	"golang.org/x/crypto/ssh"
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

func createDesiredState(config *pb.Config) *pb.Config {
	if config == nil {
		fmt.Println("Got nil, expected Config... \n Returning nil")
		return nil
	}
	//Create yaml manifest
	d := []byte(config.GetManifest())
	err := ioutil.WriteFile("manifest.yaml", d, 0644)
	if err != nil {
		log.Fatalln("Error while creating manifest.yaml file", err)
	}
	//Parse yaml to protobuf and create desiredState
	var desiredState Manifest
	yamlFile, err := ioutil.ReadFile("manifest.yaml")
	if err != nil {
		log.Fatalln("error while reading maninfest.yaml file", err)
	}
	err = yaml.Unmarshal(yamlFile, &desiredState)
	if err != nil {
		log.Fatalln("error while unmarshalling yaml file", err)
	}
	//Remove yaml manifest after loading
	err = os.Remove("manifest.yaml")
	if err != nil {
		log.Fatalln("error while removing maninfest.yaml file", err)
	}

	var clusters []*pb.Cluster
	for i, cluster := range desiredState.Clusters {
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

		// Check if a cluster has already a RSA key pair, if no generate one
		if len(config.GetCurrentState().Clusters) > i {
			if config.GetCurrentState().Clusters[i] == nil {
				privateKey, publicKey := MakeSSHKeyPair()
				cluster.PrivateKey = privateKey
				cluster.PublicKey = publicKey
			}
		} else {
			privateKey, publicKey := MakeSSHKeyPair()
			cluster.PrivateKey = privateKey
			cluster.PublicKey = publicKey
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

	return &pb.Config{
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
	}
}

// healthCheck function is function used for querring readiness/liviness of the pod running this microservice
func healthCheck() error {
	res := createDesiredState(nil)
	if res != nil {
		return fmt.Errorf("health check function got unexpected result")
	}
	return nil
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

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	// Initilize health probes
	healthChecker := healthcheck.NewClientHealthChecker("50056", healthCheck)
	healthChecker.StartProbes()

	go func() {
		// Infinite FOR loop gets config from the context box queue
		for {
			res, err := cbox.GetConfigScheduler(c)
			if err != nil {
				log.Fatalln("Error while getting config from the Scheduler", err)
			}
			if res.GetConfig() != nil {
				config := res.GetConfig()
				config = createDesiredState(config)
				fmt.Println(config.GetDesiredState())
				err = cbox.SaveConfigScheduler(c, &pb.SaveConfigRequest{Config: config})
				if err != nil {
					log.Fatalln("Error while saving the config", err)
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()
	<-ch
	fmt.Println("Stopping Scheduler")
}
