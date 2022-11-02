package main

import (
	"fmt"

	"github.com/Berops/claudie/internal/manifest"
	"github.com/Berops/claudie/internal/utils"
	"github.com/Berops/claudie/proto/pb"
)

const hostnameHashLength = 17

// createLBCluster reads manifest state and create loadbalancer clusters based on it
// returns slice of *pb.LBcluster if successful, nil otherwise
func createLBCluster(manifestState *manifest.Manifest) ([]*pb.LBcluster, error) {
	var lbClusters []*pb.LBcluster
	for _, lbCluster := range manifestState.LoadBalancer.Clusters {
		dns, err := getDNS(lbCluster.DNS, manifestState)
		if err != nil {
			return nil, fmt.Errorf("error while processing %s : %w", lbCluster.Name, err)
		}
		role, err := getMatchingRoles(manifestState.LoadBalancer.Roles, lbCluster.Roles)
		if err != nil {
			return nil, fmt.Errorf("error while processing %s : %w", lbCluster.Name, err)
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

// updateLBClusters updates the desired state of the loadbalancer clusters based on the current state
// returns error if failed, nil otherwise
func updateLBClusters(newConfig *pb.Config) error {
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
		return nil, fmt.Errorf("DNS zone not provided")
	} else {
		provider, err := manifestState.GetProvider(lbDNS.Provider)
		if err != nil {
			return nil, fmt.Errorf("Provider %s was not found", lbDNS.Provider)
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
					Port:       int32(role.Port),
					TargetPort: int32(role.TargetPort),
					Target:     target,
					RoleType:   roleType,
				}
				matchingRoles = append(matchingRoles, newRole)
			}
		}
	}
	return matchingRoles, nil
}
