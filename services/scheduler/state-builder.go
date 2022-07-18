package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/services/scheduler/manifest"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

const (
	hostnameHashLength = 17
	apiserverPort      = 6443
	gcpProvider        = "gcp"

	ALL_NODES     = "k8sAllNodes"
	CONTROL_PLANE = "k8sControlPlane"
	COMPUTE_PLANE = "k8sComputePlane"
)

//keyPair is a struct containing private and public keys as a string
type keyPair struct {
	public  string
	private string
}

//createDesiredState is a function which creates desired state based on the manifest state
//returns *pb.Config fo desired state if successful, error otherwise
func createDesiredState(config *pb.Config) (*pb.Config, error) {
	if config == nil {
		return nil, fmt.Errorf("createDesiredState got nil Config")
	}
	//read manifest state
	manifestState, err := readManifest(config)
	if err != nil {
		return nil, fmt.Errorf("error while parsing manifest from config %s : %v ", config.Name, err)
	}

	//create clusters
	k8sClusters, err := createK8sCluster(manifestState)
	if err != nil {
		return nil, fmt.Errorf("error while creating kubernetes clusters for config %s : %v", config.Name, err)
	}
	lbClusters, err := createLBCluster(manifestState)
	if err != nil {
		return nil, fmt.Errorf("error while creating Loadbalancer clusters for config %s : %v", config.Name, err)
	}

	//create new config for desired state
	newConfig := &pb.Config{
		Id:       config.GetId(),
		Name:     config.GetName(),
		Manifest: config.GetManifest(),
		DesiredState: &pb.Project{
			Name:                 manifestState.Name,
			Clusters:             k8sClusters,
			LoadBalancerClusters: lbClusters,
		},
		CurrentState: config.GetCurrentState(),
		MsChecksum:   config.GetMsChecksum(),
		DsChecksum:   config.GetDsChecksum(),
		CsChecksum:   config.GetCsChecksum(),
		BuilderTTL:   config.GetBuilderTTL(),
		SchedulerTTL: config.GetSchedulerTTL(),
	}

	//update info from current state into the desired state
	updateK8sClusters(newConfig)
	err = updateLBClusters(newConfig)
	if err != nil {
		return nil, fmt.Errorf("error while updating Loadbalancer clusters for config %s : %v", config.Name, err)
	}

	return newConfig, nil
}

//readManifest will read manifest from config and return it in manifest.Manifest struct
//returns *manifest.Manifest if sucessful, error otherwise
func readManifest(config *pb.Config) (*manifest.Manifest, error) {
	d := []byte(config.GetManifest())
	// Parse yaml to protobuf and create desiredState
	var desiredState *manifest.Manifest
	err := yaml.Unmarshal(d, desiredState)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling yaml manifest: %v", err)
	}
	return desiredState, nil
}

//updateClusterInfo updates the desired state based on the current state
// namely:
//- Hash
//- Public key
//- Private key
func updateClusterInfo(desired, current *pb.ClusterInfo) {
	desired.Hash = current.Hash
	desired.PublicKey = current.PublicKey
	desired.PrivateKey = current.PrivateKey
}

// createNodepools will create a pb.Nodepool structs based on the manifest specification
// returns error if nodepool/provider not defined, nil otherwise
func createNodepools(pools []string, desiredState *manifest.Manifest, isControl bool) ([]*pb.NodePool, error) {
	var nodePools []*pb.NodePool
	for _, nodePoolName := range pools {
		// Check if the nodepool is part of the cluster
		nodePool := findNodePool(nodePoolName, desiredState.NodePools.Dynamic)
		if nodePool != nil {
			providerName, region, zone := getProviderRegionAndZone(nodePool.Provider)
			provider := findProvider(providerName, desiredState.Providers)
			if provider == nil {
				return nil, fmt.Errorf("provider %s not defined", providerName)
			}
			nodePools = append(nodePools, &pb.NodePool{
				Name:       nodePool.Name,
				Region:     region,
				Zone:       zone,
				ServerType: nodePool.ServerType,
				Image:      nodePool.Image,
				DiskSize:   uint32(nodePool.DiskSize),
				Count:      uint32(nodePool.Count),
				Provider: &pb.Provider{
					Name:        provider.Name,
					Credentials: fmt.Sprint(provider.Credentials),
					Project:     provider.GCPProject,
				},
				IsControl: isControl,
			})
		} else {
			return nil, fmt.Errorf("nodepool %s not defined", nodePoolName)
		}
	}
	return nodePools, nil
}

//getProviderRegionAndZone will return a provider name, region and zone based on the value read from manifest
func getProviderRegionAndZone(providerMap map[string]map[string]string) (string, string, string) {
	var providerName string
	//since we cannot retrieve a key from a map without a loop in go and we know first map will have just one key,
	//we can find provider name by calling range on the first map
	for providerName = range providerMap {
	}
	return providerName, providerMap[providerName]["region"], providerMap[providerName]["zone"]
}

//findNodePool will search for the nodepool in manifest.DynamicNodePool based on the nodepool name
//returns *manifest.DynamicNodePool if found, nil otherwise
func findNodePool(nodePoolName string, nodePools []manifest.DynamicNodePool) *manifest.DynamicNodePool {
	for _, nodePool := range nodePools {
		if nodePool.Name == nodePoolName {
			return &nodePool
		}
	}
	return nil
}

//findProvider will search for the provider in manifest.Provider based on the provider name
//returns *manifest.Provider if found, nil otherwise
func findProvider(providerName string, providers []manifest.Provider) *manifest.Provider {
	for _, provider := range providers {
		if provider.Name == providerName {
			return &provider
		}
	}
	return nil
}

//createKeys will create a RSA keypair and save it into the clusterInfo provided
//return error if key creation fails
func createKeys(desiredInfo *pb.ClusterInfo) error {
	// no current cluster found with matching name, create keys/hash
	if desiredInfo.PublicKey == "" {
		keys, err := makeSSHKeyPair()
		if err != nil {
			return fmt.Errorf("error while filling up the keys for %s : %v", desiredInfo.Name, err)
		}
		desiredInfo.PrivateKey = keys.private
		desiredInfo.PublicKey = keys.public
	}
	return nil
}

//makeSSHKeyPair function generates SSH key pair
//returns key pair if successful, nil otherwise
func makeSSHKeyPair() (keyPair, error) {
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
