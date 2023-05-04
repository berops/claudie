package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

// keyPair is a struct containing private and public SSH keys as a string.
// These SSH key-pairs are required to SSH into the VMs in the cluster and execute commands.
type keyPair struct {
	public  string
	private string
}

const hostnameHashLength = 17

// CreateLBCluster reads manifest state and create loadbalancer clusters based on it
// returns slice of *pb.LBcluster if successful, nil otherwise
func CreateLBCluster(manifestState *manifest.Manifest) ([]*pb.LBcluster, error) {
	var lbClusters []*pb.LBcluster
	for _, lbCluster := range manifestState.LoadBalancer.Clusters {
		dns, err := getDNS(lbCluster.DNS, manifestState)
		if err != nil {
			return nil, fmt.Errorf("error while building desired state for LB %s : %w", lbCluster.Name, err)
		}
		role, err := getMatchingRoles(manifestState.LoadBalancer.Roles, lbCluster.Roles)
		if err != nil {
			return nil, fmt.Errorf("error while building desired state for LB %s : %w", lbCluster.Name, err)
		}
		newLbCluster := &pb.LBcluster{
			ClusterInfo: &pb.ClusterInfo{
				Name: lbCluster.Name,
				Hash: utils.CreateHash(utils.HashLength),
			},
			Roles:       role,
			Dns:         dns,
			TargetedK8S: lbCluster.TargetedK8s,
		}
		nodes, err := manifestState.CreateNodepools(lbCluster.Pools, false)
		if err != nil {
			return nil, fmt.Errorf("error while creating nodepools for %s : %w", lbCluster.Name, err)
		}
		newLbCluster.ClusterInfo.NodePools = nodes
		lbClusters = append(lbClusters, newLbCluster)
	}
	return lbClusters, nil
}

// UpdateLBClusters updates the desired state of the loadbalancer clusters based on the current state
// returns error if failed, nil otherwise
func UpdateLBClusters(newConfig *pb.Config) error {
clusterLbDesired:
	for _, clusterLbDesired := range newConfig.DesiredState.LoadBalancerClusters {
		for _, clusterLbCurrent := range newConfig.CurrentState.LoadBalancerClusters {
			// found current cluster with matching name
			if clusterLbDesired.ClusterInfo.Name == clusterLbCurrent.ClusterInfo.Name {
				updateClusterInfo(clusterLbDesired.ClusterInfo, clusterLbCurrent.ClusterInfo)
				// copy hostname from current state if not specified in manifest
				if clusterLbDesired.Dns.Hostname == "" {
					clusterLbDesired.Dns.Hostname = clusterLbCurrent.Dns.Hostname
				}
				//skip the checks
				continue clusterLbDesired
			}
		}
		// no current cluster found with matching name, create keys
		if clusterLbDesired.ClusterInfo.PublicKey == "" {
			err := createKeys(clusterLbDesired.ClusterInfo)
			if err != nil {
				return fmt.Errorf("error encountered while creating desired state for %s : %w", clusterLbDesired.ClusterInfo.Name, err)
			}
		}
		// create hostname if its not set and not present in current state
		if clusterLbDesired.Dns.Hostname == "" {
			clusterLbDesired.Dns.Hostname = utils.CreateHash(hostnameHashLength)
		}
	}
	return nil
}

// getDNS reads manifest state and returns *pb.DNS based on it
// return *pb.DNS if successful, error if provider has not been found
func getDNS(lbDNS manifest.DNS, manifestState *manifest.Manifest) (*pb.DNS, error) {
	if lbDNS.DNSZone == "" {
		return nil, fmt.Errorf("DNS zone not provided in manifest %s", manifestState.Name)
	} else {
		provider, err := manifestState.GetProvider(lbDNS.Provider)
		if err != nil {
			return nil, fmt.Errorf("provider %s was not found in manifest %s", lbDNS.Provider, manifestState.Name)
		}
		return &pb.DNS{
			DnsZone:  lbDNS.DNSZone,
			Provider: provider,
			Hostname: lbDNS.Hostname,
		}, nil
	}
}

// getMatchingRoles will read roles from manifest state and returns slice of *pb.Role
// returns slice of *[]pb.Roles if successful, error if Target from manifest state not found
func getMatchingRoles(roles []manifest.Role, roleNames []string) ([]*pb.Role, error) {
	var matchingRoles []*pb.Role

	for _, roleName := range roleNames {
		for _, role := range roles {
			if role.Name == roleName {
				// find numeric value of a pb.Target specified
				t, ok := pb.Target_value[role.Target]
				if !ok {
					return nil, fmt.Errorf("target %s not found", role.Target)
				}
				//parse to the pb.Target type
				target := pb.Target(t)
				var roleType pb.RoleType

				if role.TargetPort == manifest.APIServerPort && target == pb.Target_k8sControlPlane {
					roleType = pb.RoleType_ApiServer
				} else {
					roleType = pb.RoleType_Ingress
				}

				newRole := &pb.Role{
					Name:       role.Name,
					Protocol:   role.Protocol,
					Port:       role.Port,
					TargetPort: role.TargetPort,
					Target:     target,
					RoleType:   roleType,
				}
				matchingRoles = append(matchingRoles, newRole)
			}
		}
	}
	return matchingRoles, nil
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
