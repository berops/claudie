package utils

import (
	"fmt"

	"github.com/berops/claudie/internal/manifest"
	"github.com/berops/claudie/internal/utils"
	"github.com/berops/claudie/proto/pb"
)

const hostnameHashLength = 17

// CreateLBCluster reads the unmarshalled manifest and creates loadbalancer clusters based on it.
// Returns slice of *pb.LBcluster if successful, nil otherwise along with the error.
func CreateLBCluster(unmarshalledManifest *manifest.Manifest) ([]*pb.LBcluster, error) {
	var lbClusters []*pb.LBcluster
	for _, lbCluster := range unmarshalledManifest.LoadBalancer.Clusters {
		dns, err := getDNS(lbCluster.DNS, unmarshalledManifest)
		if err != nil {
			return nil, fmt.Errorf("error while building desired state for LB %s : %w", lbCluster.Name, err)
		}
		attachedRoles := getRolesAttachedToLBCluster(unmarshalledManifest.LoadBalancer.Roles, lbCluster.Roles)

		newLbCluster := &pb.LBcluster{
			ClusterInfo: &pb.ClusterInfo{
				Name: lbCluster.Name,
				Hash: utils.CreateHash(utils.HashLength),
			},
			Roles:       attachedRoles,
			Dns:         dns,
			TargetedK8S: lbCluster.TargetedK8s,
		}
		nodes, err := unmarshalledManifest.CreateNodepools(lbCluster.Pools, false)
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
					clusterLbDesired.Dns.Endpoint = clusterLbCurrent.Dns.Endpoint
				}
				//skip the checks
				continue clusterLbDesired
			}
		}
		// no current cluster found with matching name, create keys
		if clusterLbDesired.ClusterInfo.PublicKey == "" {
			err := createSSHKeyPair(clusterLbDesired.ClusterInfo)
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

// getDNS reads the unmarshalled manifest and returns *pb.DNS based on it.
// Return *pb.DNS if successful, error if provider has not been found.
func getDNS(lbDNS manifest.DNS, unmarshalledManifest *manifest.Manifest) (*pb.DNS, error) {
	if lbDNS.DNSZone == "" {
		return nil, fmt.Errorf("DNS zone not provided in manifest %s", unmarshalledManifest.Name)
	} else {
		provider, err := unmarshalledManifest.GetProvider(lbDNS.Provider)
		if err != nil {
			return nil, fmt.Errorf("provider %s was not found in manifest %s", lbDNS.Provider, unmarshalledManifest.Name)
		}
		return &pb.DNS{
			DnsZone:  lbDNS.DNSZone,
			Provider: provider,
			Hostname: lbDNS.Hostname,
		}, nil
	}
}

// getRolesAttachedToLBCluster will read roles attached to the LB cluster from the unmarshalled manifest and return them.
// Returns slice of *[]pb.Roles if successful, error if Target from manifest state not found
func getRolesAttachedToLBCluster(roles []manifest.Role, roleNames []string) []*pb.Role {
	var matchingRoles []*pb.Role

	for _, roleName := range roleNames {
		for _, role := range roles {
			if role.Name == roleName {
				var roleType pb.RoleType

				// The manifest validation is handling the check if the target nodepools of the
				// role are control nodepools and thus can be used as a valid API loadbalancer.
				// Given this invariant we can simply check for the port.
				if role.TargetPort == manifest.APIServerPort {
					roleType = pb.RoleType_ApiServer
				} else {
					roleType = pb.RoleType_Ingress
				}

				newRole := &pb.Role{
					Name:        role.Name,
					Protocol:    role.Protocol,
					Port:        role.Port,
					TargetPort:  role.TargetPort,
					TargetPools: role.TargetPools,
					RoleType:    roleType,
				}
				matchingRoles = append(matchingRoles, newRole)
			}
		}
	}

	return matchingRoles
}
