package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/proto/pb"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

// keyPair is a struct containing private and public keys as a string
type keyPair struct {
	public  string
	private string
}

// createDesiredState is a function which creates desired state based on the manifest state
// returns *pb.Config fo desired state if successful, error otherwise
func createDesiredState(config *pb.Config) (*pb.Config, error) {
	if config == nil {
		return nil, fmt.Errorf("createDesiredState got nil Config")
	}

	// check if the manifest string is empty and set DesiredState to nil
	if config.Manifest == "" {
		return &pb.Config{
			Id:           config.GetId(),
			Name:         config.GetName(),
			Manifest:     config.GetManifest(),
			DesiredState: nil,
			CurrentState: config.GetCurrentState(),
			MsChecksum:   config.GetMsChecksum(),
			DsChecksum:   config.GetDsChecksum(),
			CsChecksum:   config.GetCsChecksum(),
			BuilderTTL:   config.GetBuilderTTL(),
			SchedulerTTL: config.GetSchedulerTTL(),
		}, nil
	}

	//read manifest state
	manifestState, err := readManifest(config)
	if err != nil {
		return nil, fmt.Errorf("error while parsing manifest from config %s : %w ", config.Name, err)
	}

	//create clusters
	k8sClusters, err := createK8sCluster(manifestState)
	if err != nil {
		return nil, fmt.Errorf("error while creating kubernetes clusters for config %s : %w", config.Name, err)
	}
	lbClusters, err := createLBCluster(manifestState)
	if err != nil {
		return nil, fmt.Errorf("error while creating Loadbalancer clusters for config %s : %w", config.Name, err)
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
	err = updateK8sClusters(newConfig)
	if err != nil {
		return nil, fmt.Errorf("error while updating Kubernetes clusters for config %s : %w", config.Name, err)
	}
	err = updateLBClusters(newConfig)
	if err != nil {
		return nil, fmt.Errorf("error while updating Loadbalancer clusters for config %s : %w", config.Name, err)
	}

	return newConfig, nil
}

// readManifest will read manifest from config and return it in manifest.Manifest struct
// returns *manifest.Manifest if successful, error otherwise
func readManifest(config *pb.Config) (*manifest.Manifest, error) {
	d := []byte(config.GetManifest())
	// Parse yaml to protobuf and create desiredState
	var desiredState manifest.Manifest
	err := yaml.Unmarshal(d, &desiredState)
	if err != nil {
		return nil, fmt.Errorf("error while unmarshalling yaml manifest for config %s: %w", config.Name, err)
	}
	return &desiredState, nil
}

// updateClusterInfo updates the desired state based on the current state
// namely:
// - Hash
// - Public key
// - Private key
// - AutoscalerConfig
// - existing nodes
// - nodepool metadata
func updateClusterInfo(desired, current *pb.ClusterInfo) {
	desired.Hash = current.Hash
	desired.PublicKey = current.PublicKey
	desired.PrivateKey = current.PrivateKey
	// check for autoscaler configuration
desired:
	for _, desiredNp := range desired.NodePools {
		for _, currentNp := range current.NodePools {
			// Found nodepool in desired and in Current
			if desiredNp.Name == currentNp.Name {
				// Save current nodes and metadata
				desiredNp.Nodes = currentNp.Nodes
				desiredNp.Metadata = currentNp.Metadata
				// Update the count
				if currentNp.AutoscalerConfig != nil && desiredNp.AutoscalerConfig != nil {
					// Both have Autoscaler conf defined, use same count as in current
					desiredNp.Count = currentNp.Count
				} else if currentNp.AutoscalerConfig == nil && desiredNp.AutoscalerConfig != nil {
					// Desired is autoscaled, but not current
					if desiredNp.AutoscalerConfig.Min > currentNp.Count {
						// Cannot have fewer nodes than defined min
						desiredNp.Count = desiredNp.AutoscalerConfig.Min
					} else if desiredNp.AutoscalerConfig.Max < currentNp.Count {
						// Cannot have more nodes than defined max
						desiredNp.Count = desiredNp.AutoscalerConfig.Max
					} else {
						// Use same count as in current for now, autoscaler might change it later
						desiredNp.Count = currentNp.Count
					}
				}
				continue desired
			}
		}
	}
}

// createKeys will create a RSA key-pair and save it into the clusterInfo provided
// return error if key creation fails
func createKeys(desiredInfo *pb.ClusterInfo) error {
	// no current cluster found with matching name, create keys/hash
	if desiredInfo.PublicKey == "" {
		keys, err := makeSSHKeyPair()
		if err != nil {
			return fmt.Errorf("error while creating keys for %s : %w", desiredInfo.Name, err)
		}
		desiredInfo.PrivateKey = keys.private
		desiredInfo.PublicKey = keys.public
	}
	return nil
}

// makeSSHKeyPair function generates SSH key pair
// returns key pair if successful, nil otherwise
func makeSSHKeyPair() (keyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
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
