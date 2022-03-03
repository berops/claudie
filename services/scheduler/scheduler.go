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

const (
	defaultSchedulerPort = 50056
	apiserverPort        = 6443
	gcpProvider          = "gcp"
)

var claudieProvider = &pb.Provider{
	Name:        "gcp",
	Credentials: "../../../../../keys/platform-infrastructure-316112-bd7953f712df.json",
}

var DefaultDNS = &pb.DNS{
	DnsZone:  "lb-zone",
	Project:  "platform-infrastructure-316112",
	Provider: claudieProvider,
}

////////////////////YAML STRUCT//////////////////////////////////////////////////

type Manifest struct {
	Name         string       `yaml:"name"`
	Providers    []Provider   `yaml:"providers"`
	NodePools    NodePool     `yaml:"nodePools"`
	Kubernetes   Kubernetes   `yaml:"kubernetes"`
	LoadBalancer LoadBalancer `yaml:"loadBalancers"`
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
	Name       string `yaml:"name"`
	Protocol   string `yaml:"protocol"`
	Port       int32  `yaml:"port"`
	TargetPort int32  `yaml:"target_port"`
	Target     string `yaml:"target"`
}

type LoadBalancerCluster struct {
	Name        string   `yaml:"name"`
	Roles       []string `yaml:"roles"`
	DNS         DNS      `yaml:"dns,omitempty"`
	TargetedK8s string   `yaml:"targeted-k8s"`
	Pools       []string `yaml:"pools"`
}

type DNS struct {
	DNSZone  string `yaml:"dns_zone,omitempty"`
	Project  string `yaml:"project,omitempty"`
	Hostname string `yaml:"hostname,omitempty"`
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

	var clusters []*pb.K8Scluster
	for _, cluster := range desiredState.Kubernetes.Clusters {

		newCluster := &pb.K8Scluster{
			ClusterInfo: &pb.ClusterInfo{
				Name: strings.ToLower(cluster.Name),
				Hash: utils.CreateHash(utils.HashLength),
			},
			Kubernetes: cluster.Version,
			Network:    cluster.Network,
		}

		var ComputeNodePools, ControlNodePools []*pb.NodePool

		// Control nodePool
		ControlNodePools = createNodepools(cluster.Pools.Control, desiredState, true)
		// compute nodepools
		ComputeNodePools = createNodepools(cluster.Pools.Compute, desiredState, false)

		newCluster.ClusterInfo.NodePools = append(ControlNodePools, ComputeNodePools...)
		clusters = append(clusters, newCluster)
	}

	var lbClusters []*pb.LBcluster
	for _, lbCluster := range desiredState.LoadBalancer.Clusters {

		newLbCluster := &pb.LBcluster{
			ClusterInfo: &pb.ClusterInfo{
				Name: lbCluster.Name,
				Hash: utils.CreateHash(utils.HashLength),
			},
			Roles:       getMatchingRoles(desiredState.LoadBalancer.Roles, lbCluster.Roles),
			Dns:         getDNS(lbCluster.DNS, desiredState.Providers),
			TargetedK8S: lbCluster.TargetedK8s,
		}

		newLbCluster.ClusterInfo.NodePools = createNodepools(lbCluster.Pools, desiredState, false)
		lbClusters = append(lbClusters, newLbCluster)
	}

	res := &pb.Config{
		Id:       config.GetId(),
		Name:     config.GetName(),
		Manifest: config.GetManifest(),
		DesiredState: &pb.Project{
			Name:                 desiredState.Name,
			Clusters:             clusters,
			LoadBalancerClusters: lbClusters,
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
			if clusterDesired.ClusterInfo.Name == clusterCurrent.ClusterInfo.Name {
				if clusterCurrent.ClusterInfo.PublicKey != "" {
					clusterDesired.ClusterInfo.PublicKey = clusterCurrent.ClusterInfo.PublicKey
					clusterDesired.ClusterInfo.PrivateKey = clusterCurrent.ClusterInfo.PrivateKey
				}
				if clusterCurrent.ClusterInfo.Hash != "" {
					clusterDesired.ClusterInfo.Hash = clusterCurrent.ClusterInfo.Hash
				}
				if clusterCurrent.Kubeconfig != "" {
					clusterDesired.Kubeconfig = clusterCurrent.Kubeconfig
				}
				//skip the checks bellow
				continue clusterDesired
			}
		}
		// no current cluster found with matching name, create keys/hash
		if clusterDesired.ClusterInfo.PublicKey == "" {
			privateKey, publicKey := MakeSSHKeyPair()
			clusterDesired.ClusterInfo.PrivateKey = privateKey
			clusterDesired.ClusterInfo.PublicKey = publicKey
		}
	}

clusterLbDesired:
	for _, clusterLbDesired := range res.DesiredState.LoadBalancerClusters {
		for _, clusterLbCurrent := range res.CurrentState.LoadBalancerClusters {
			// found current cluster with matching name
			if clusterLbDesired.ClusterInfo.Name == clusterLbCurrent.ClusterInfo.Name {
				if clusterLbCurrent.ClusterInfo.PublicKey != "" {
					clusterLbDesired.ClusterInfo.PublicKey = clusterLbCurrent.ClusterInfo.PublicKey
					clusterLbDesired.ClusterInfo.PrivateKey = clusterLbCurrent.ClusterInfo.PrivateKey
				}
				if clusterLbDesired.ClusterInfo.Hash != "" {
					clusterLbDesired.ClusterInfo.Hash = clusterLbCurrent.ClusterInfo.Hash
				}
				//skip the checks bellow
				continue clusterLbDesired
			}
		}
		// no current cluster found with matching name, create keys/hash
		if clusterLbDesired.ClusterInfo.PublicKey == "" {
			privateKey, publicKey := MakeSSHKeyPair()
			clusterLbDesired.ClusterInfo.PrivateKey = privateKey
			clusterLbDesired.ClusterInfo.PublicKey = publicKey
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
	fmt.Println(config)

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
			go func(config *pb.Config) {
				log.Info().Msgf("Processing %s ", config.Name)
				err := processConfig(config, c)
				if err != nil {
					log.Info().Msgf("scheduler:processConfig failed: %s", err)
					//save error message to config
					errSave := saveErrorMessage(config, c, err)
					if errSave != nil {
						log.Error().Msgf("scheduler:failed to save error to the config: %s : processConfig failed: %s", errSave, err)
					}
				}
			}(config)
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

func getMatchingRoles(roles []Role, roleNames []string) []*pb.Role {
	var matchingRoles []*pb.Role

	for _, roleName := range roleNames {
		for _, role := range roles {
			if role.Name == roleName {

				var target pb.Target
				var roleType pb.RoleType

				if role.Target == "k8sAllNodes" {
					target = pb.Target_k8sAllNodes
				} else if role.Target == "k8sControlPlane" {
					target = pb.Target_k8sControlPlane
				} else if role.Target == "k8sComputePlane" {
					target = pb.Target_k8sComputePlane
				}

				if role.TargetPort == apiserverPort && target == pb.Target_k8sControlPlane {
					roleType = pb.RoleType_ApiServer
				} else {
					roleType = pb.RoleType_Ingress
				}

				newRole := &pb.Role{
					Name:       role.Name,
					Protocol:   role.Protocol,
					Port:       int32(role.Port),
					TargetPort: int32(role.TargetPort),
					Target:     target,
					RoleType:   roleType,
				}
				matchingRoles = append(matchingRoles, newRole)
			}
		}
	}
	return matchingRoles
}

func getDNS(lbDNS DNS, provider []Provider) *pb.DNS {
	if lbDNS.DNSZone == "" {
		return DefaultDNS // default zone is used
	} else {
		providerIndex := searchProvider(gcpProvider, provider)
		return &pb.DNS{
			DnsZone: lbDNS.DNSZone,
			Provider: &pb.Provider{
				Name:        provider[providerIndex].Name,
				Credentials: provider[providerIndex].Credentials,
			},
			Project:  lbDNS.Project,
			Hostname: lbDNS.Hostname,
		}
	}
}

// function saveErrorMessage saves error message to config
func saveErrorMessage(config *pb.Config, c pb.ContextBoxServiceClient, err error) error {
	config.CurrentState = config.DesiredState // Update currentState, so we can use it for deletion later
	config.ErrorMessage = err.Error()
	errSave := cbox.SaveConfigScheduler(c, &pb.SaveConfigRequest{Config: config})
	if errSave != nil {
		return fmt.Errorf("error while saving the config: %v", err)
	}
	return nil
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

	// Initialize health probes
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
