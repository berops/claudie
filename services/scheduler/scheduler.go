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

	"github.com/Berops/platform/envs"
	"github.com/Berops/platform/healthcheck"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/Berops/platform/services/scheduler/manifest"
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
	hostnameHashLen      = 17
	gcpProvider          = "gcp"
)

var (
	claudieProvider = &pb.Provider{
		Name:        "gcp",
		Credentials: "../../../../../keys/platform-infrastructure-316112-bd7953f712df.json",
	}
	DefaultDNS = &pb.DNS{
		DnsZone:  "lb-zone",
		Project:  "platform-infrastructure-316112",
		Provider: claudieProvider,
	}
)

type keyPair struct {
	public  string
	private string
}

// MakeSSHKeyPair function generates SSH privateKey,publicKey pair
// returns (strPrivateKey, strPublicKey)
func MakeSSHKeyPair() (keyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2042)
	if err != nil {
		return keyPair{}, err
	}

	// generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return keyPair{}, err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return keyPair{}, err
	}

	var pubKeyBuf strings.Builder
	pubKeyBuf.Write(ssh.MarshalAuthorizedKey(pub))

	return keyPair{public: pubKeyBuf.String(), private: privKeyBuf.String()}, nil
}

func createDesiredState(config *pb.Config) (*pb.Config, error) {
	if config == nil {
		return nil, fmt.Errorf("createDesiredState got nil Config")
	}
	d := []byte(config.GetManifest())
	// Parse yaml to protobuf and create desiredState
	var desiredState manifest.Manifest
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
				fillExistingKeys(clusterDesired.ClusterInfo, clusterCurrent.ClusterInfo)
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
			err := createKeys(clusterDesired.ClusterInfo)
			if err != nil {
				log.Error().Msgf("Error encountered while creating desired state for %s : %v", clusterDesired.ClusterInfo.Name, err)
			}
		}
	}

clusterLbDesired:
	for _, clusterLbDesired := range res.DesiredState.LoadBalancerClusters {
		for _, clusterLbCurrent := range res.CurrentState.LoadBalancerClusters {
			// found current cluster with matching name
			if clusterLbDesired.ClusterInfo.Name == clusterLbCurrent.ClusterInfo.Name {
				fillExistingKeys(clusterLbDesired.ClusterInfo, clusterLbCurrent.ClusterInfo)
				if clusterLbDesired.ClusterInfo.Hash != "" {
					clusterLbDesired.ClusterInfo.Hash = clusterLbCurrent.ClusterInfo.Hash
				}
				// copy hostname from current state if not specified in manifest
				if clusterLbDesired.Dns.Hostname == "" {
					clusterLbDesired.Dns.Hostname = clusterLbCurrent.Dns.Hostname
				}

				//skip the checks
				continue clusterLbDesired
			}
		}
		// no current cluster found with matching name, create keys/hash
		if clusterLbDesired.ClusterInfo.PublicKey == "" {
			err := createKeys(clusterLbDesired.ClusterInfo)
			if err != nil {
				log.Error().Msgf("Error encountered while creating desired state for %s : %v", clusterLbDesired.ClusterInfo.Name, err)
			}
		}
		// create hostname if its not set and not present in current state
		if clusterLbDesired.Dns.Hostname == "" {
			clusterLbDesired.Dns.Hostname = utils.CreateHash(hostnameHashLen)
		}
	}

	return res, nil
}

// function fillExistingKey will copy the keys from currentState to desired state
func fillExistingKeys(desiredInfo, currentInfo *pb.ClusterInfo) {
	if currentInfo.PublicKey != "" {
		desiredInfo.PublicKey = currentInfo.PublicKey
		desiredInfo.PrivateKey = currentInfo.PrivateKey
	}
}

// function createKeys will create a RSA keypair and save it into the desired state
// return error if key creation fails
func createKeys(desiredInfo *pb.ClusterInfo) error {
	// no current cluster found with matching name, create keys/hash
	if desiredInfo.PublicKey == "" {
		keys, err := MakeSSHKeyPair()
		if err != nil {
			return fmt.Errorf("error while filling up the keys for %s : %v", desiredInfo.Name, err)
		}
		desiredInfo.PrivateKey = keys.private
		desiredInfo.PublicKey = keys.public
	}
	return nil
}

// populate nodepools for a cluster
func createNodepools(pools []string, desiredState manifest.Manifest, isControl bool) []*pb.NodePool {
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
					Credentials: fmt.Sprint(desiredState.Providers[providerIndex].Credentials),
					Project:     desiredState.Providers[providerIndex].GCPProject,
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
	// fmt.Println(config)
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
func searchNodePool(nodePoolName string, nodePools []manifest.DynamicNodePool) (bool, int) {
	for index, nodePool := range nodePools {
		if nodePool.Name == nodePoolName {
			return true, index
		}
	}
	return false, -1
}

func searchProvider(providerName string, providers []manifest.Provider) int {
	for index, provider := range providers {
		if provider.Name == providerName {
			return index
		}
	}
	return -1
}

func getMatchingRoles(roles []manifest.Role, roleNames []string) []*pb.Role {
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

func getDNS(lbDNS manifest.DNS, provider []manifest.Provider) *pb.DNS {
	if lbDNS.DNSZone == "" {
		return DefaultDNS // default zone is used
	} else {
		providerIndex := searchProvider(gcpProvider, provider)
		return &pb.DNS{
			DnsZone: lbDNS.DNSZone,
			Provider: &pb.Provider{
				Name:        provider[providerIndex].Name,
				Credentials: fmt.Sprint(provider[providerIndex].Credentials),
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
	utils.InitLog("scheduler")

	// Create connection to Context-box
	log.Info().Msgf("Dial Context-box: %s", envs.ContextBoxURL)
	cc, err := utils.GrpcDialWithInsecure("context-box", envs.ContextBoxURL)
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
