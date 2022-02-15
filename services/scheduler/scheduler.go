package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/urls"
	"github.com/Berops/platform/utils"
	"github.com/Berops/platform/worker"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

const defaultSchedulerPort = 50056

////////////////////YAML STRUCT//////////////////////////////////////////////////

type Manifest struct {
	Name         string       `yaml:"name"`
	Providers    []Provider   `yaml:"providers"`
	NodePools    NodePool     `yaml:"nodePools"`
	LoadBalancer LoadBalancer `yaml:"loadBalancer"`
	Kubernetes   Kubernetes   `yaml:"kubernetes"`
}

type Provider struct {
	Name        string `yaml:"name"`
	Credentials string `yaml:"credentials"`
}

type NodePool struct {
	Dynamic []DynamicNodePool `yaml:"dynamic"`
	Static  []StaticNodePool  `yaml:"static"`
}

type LoadBalancer struct {
	Roles    []Role                `yaml:"roles"`
	Clusters []LoadBalancerCluster `yaml:"clusters"`
}

type Kubernetes struct {
	Clusters []Cluster `yaml:"clusters"`
}

type DynamicNodePool struct {
	Name       string                       `yaml:"name"`
	Provider   map[string]map[string]string `yaml:"provider"`
	Count      int64                        `yaml:"count"`
	ServerType string                       `yaml:"server_type"`
	Image      string                       `yaml:"image"`
	Datacenter string                       `yaml:"datacenter"`
	DiskSize   int64                        `yaml:"disk_size"`
}

type StaticNodePool struct {
	Name  string `yaml:"name"`
	Nodes []Node `yaml:"nodes"`
}

type Node struct {
	PublicIP      string `yaml:"publicIP"`
	PrivateSSHKey string `yaml:"privateSshKey"`
}

type Cluster struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Network string `yaml:"network"`
	Pools   Pool   `yaml:"pools"`
}

type Pool struct {
	Control []string `yaml:"control"`
	Compute []string `yaml:"compute"`
}

type Role struct {
	Name string `yaml:"name"`
	Conf Conf   `yaml:"conf"`
}

type Conf struct {
	Protocol   string `yaml:"protocol"`
	Port       uint32 `yaml:"port"`
	TargetPort uint32 `yaml:"targetPort"`
}

type LoadBalancerCluster struct {
	Name   string   `yaml:"name"`
	Role   string   `yaml:"role"`
	DNS    DNS      `yaml:"dns"`
	Target Target   `yaml:"target"`
	Pools  []string `yaml:"pools"`
}

type DNS struct {
	Hostname string   `yaml:"hostname"`
	Provider []string `yaml:"provider"`
}

type Target struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

////////////////////////////////////////////////////////////////////////////////

// MakeSSHKeyPair function generates SSH privateKey,publicKey pair
// returns (strPrivateKey, strPublicKey)
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
		return nil, fmt.Errorf("createDesiredState got nil Config")
	}
	d := []byte(config.GetManifest())
	// Parse yaml to protobuf and create desiredState
	var desiredState Manifest
	err := yaml.Unmarshal(d, &desiredState)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling yaml manifest: %v", err)
	}

	var clusters []*pb.Cluster
	for _, cluster := range desiredState.Kubernetes.Clusters {

		newCluster := &pb.Cluster{
			Name:       strings.ToLower(cluster.Name),
			Kubernetes: cluster.Version,
			Network:    cluster.Network,
			Hash:       utils.CreateHash(utils.HashLength),
		}

		var ComputeNodePools, ControlNodePools []*pb.NodePool

		// Control nodePool
		ControlNodePools = createNodepools(cluster.Pools.Control, desiredState, true)
		// compute nodepools
		ComputeNodePools = createNodepools(cluster.Pools.Compute, desiredState, false)

		newCluster.NodePools = append(ControlNodePools, ComputeNodePools...)
		clusters = append(clusters, newCluster)
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

clusterDesired:
	for _, clusterDesired := range res.DesiredState.Clusters {
		for _, clusterCurrent := range res.CurrentState.Clusters {
			// found current cluster with matching name
			if clusterDesired.Name == clusterCurrent.Name {
				if clusterCurrent.PublicKey != "" {
					clusterDesired.PublicKey = clusterCurrent.PublicKey
					clusterDesired.PrivateKey = clusterCurrent.PrivateKey
				}
				if clusterCurrent.Hash != "" {
					clusterDesired.Hash = clusterCurrent.Hash
				}
				if clusterCurrent.Kubeconfig != "" {
					clusterDesired.Kubeconfig = clusterCurrent.Kubeconfig
				}
				//skip the checks bellow
				continue clusterDesired
			}

		}
		// no current cluster found with matching name, create keys/hash
		if clusterDesired.PublicKey == "" {
			privateKey, publicKey := MakeSSHKeyPair()
			clusterDesired.PrivateKey = privateKey
			clusterDesired.PublicKey = publicKey
		}
		if clusterDesired.Hash == "" {
			clusterDesired.Hash = utils.CreateHash(utils.HashLength)
		}
	}

	return res, nil
}

// populate nodepools for a cluster
func createNodepools(pools []string, desiredState Manifest, isControl bool) []*pb.NodePool {
	var nodePools []*pb.NodePool
	for _, nodePool := range pools {
		// Check if the nodepool is part of the cluster
		if isFound, position := searchNodePool(nodePool, desiredState.NodePools.Dynamic); isFound {

			providerName, region, zone := getProviderRegionAndZone(desiredState.NodePools.Dynamic[position].Provider)
			providerIndex := searchProvider(providerName, desiredState.Providers)
			if providerIndex < 0 {
				log.Error().Msg("Provider not defined")
				continue
			}
			nodePools = append(nodePools, &pb.NodePool{
				Name:       desiredState.NodePools.Dynamic[position].Name,
				Region:     region,
				Zone:       zone,
				ServerType: desiredState.NodePools.Dynamic[position].ServerType,
				Image:      desiredState.NodePools.Dynamic[position].Image,
				DiskSize:   uint32(desiredState.NodePools.Dynamic[position].DiskSize),
				Count:      uint32(desiredState.NodePools.Dynamic[position].Count),
				Provider: &pb.Provider{
					Name:        desiredState.Providers[providerIndex].Name,
					Credentials: desiredState.Providers[providerIndex].Credentials,
				},
				IsControl: isControl,
			})
		}
	}
	return nodePools
}

// processConfig is function used to carry out task specific to Scheduler concurrently
func processConfig(config *pb.Config, c pb.ContextBoxServiceClient) (err error) {
	log.Printf("Processing new config")
	config, err = createDesiredState(config)
	if err != nil {
		return fmt.Errorf("error while creating a desired state: %v", err)
	}

	log.Info().Interface("project", config.GetDesiredState())
	err = cbox.SaveConfigScheduler(c, &pb.SaveConfigRequest{Config: config})
	if err != nil {
		return fmt.Errorf("error while saving the config: %v", err)
	}

	return nil
}

// healthCheck function is used for querring readiness of the pod running this microservice
func healthCheck() error {
	res, err := createDesiredState(nil)
	if res != nil || err == nil {
		return fmt.Errorf("health check function got unexpected result")
	}
	return nil
}

func configProcessor(c pb.ContextBoxServiceClient) func() error {
	return func() error {
		res, err := cbox.GetConfigScheduler(c)
		if err != nil {
			return fmt.Errorf("error while getting Scheduler config: %v", err)
		}

		config := res.GetConfig()
		if config != nil {
			go func() error {
				log.Info().Msgf("Processing %s ", config.Name)
				err := processConfig(config, c)
				if err != nil {
					log.Info().Msgf("scheduler:processConfig failed: %s", err)
					return err
				}
				return nil
			}()
		}
		return nil
	}
}

func getProviderRegionAndZone(providerMap map[string]map[string]string) (string, string, string) {

	var provider string
	for provider = range providerMap {
	}
	return provider, providerMap[provider]["region"], providerMap[provider]["zone"]
}

// search of the nodePool in the nodePools []DynamicNode
func searchNodePool(nodePoolName string, nodePools []DynamicNodePool) (bool, int) {
	for index, nodePool := range nodePools {
		if nodePool.Name == nodePoolName {
			return true, index
		}
	}
	return false, -1
}

func searchProvider(providerName string, providers []Provider) int {
	for index, provider := range providers {
		if provider.Name == providerName {
			return index
		}
	}
	return -1
}

func main() {
	// initialize logger
	utils.InitLog("scheduler", "GOLANG_LOG")

	// Create connection to Context-box
	log.Info().Msgf("Dial Context-box: %s", urls.ContextBoxURL)
	cc, err := utils.GrpcDialWithInsecure("context-box", urls.ContextBoxURL)
	if err != nil {
		log.Fatal().Err(err)
	}

	defer func() { utils.CloseClientConnection(cc) }()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	// Initilize health probes
	healthChecker := healthcheck.NewClientHealthChecker(fmt.Sprint(defaultSchedulerPort), healthCheck)
	healthChecker.StartProbes()

	g, ctx := errgroup.WithContext(context.Background())
	w := worker.NewWorker(ctx, 10*time.Second, configProcessor(c), worker.ErrorLogger)

	g.Go(func() error {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		defer signal.Stop(ch)
		<-ch
		return errors.New("scheduler interrupt signal")
	})

	g.Go(func() error {
		w.Run()
		return nil
	})

	log.Info().Msgf("Stopping Scheduler: %v", g.Wait())
}
