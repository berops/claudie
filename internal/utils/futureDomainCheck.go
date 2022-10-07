package utils

import (
	"fmt"

	"github.com/Berops/claudie/proto/pb"
)

const (
	maxLength  = 80 // total length of domain = 8 + len(publicIP)[15] + 19 + len(NodeName) + margin
	baseLength = 8 + 19 + 15
)

// CheckLengthOfFutureDomain will check if the possible domain name is too long
// returns error if domain will be too long, nil if not
// Described in https://github.com/Berops/claudie/issues/112#issuecomment-1015432224
func CheckLengthOfFutureDomain(config *pb.Config) error {
	// https://<public-ip>:6443/<api-path>/<node-name>
	// <node-name> = clusterName + hash + nodeName + indexLength + separators
	for _, cluster := range config.DesiredState.GetClusters() {
		clusterNameLength := len(cluster.ClusterInfo.Name) + HashLength + 1 // "-" separator between clusterName and hash
		for _, nodepool := range cluster.ClusterInfo.GetNodePools() {
			nodeNameLength := len(fmt.Sprint(nodepool.Count)) + len(nodepool.Name) + 2 // "-" separator between hash and nodeName AND "-" separator between nodeName and Count
			if maxLength <= clusterNameLength+nodeNameLength+baseLength {
				return fmt.Errorf("cluster name %s or nodepool name %s is too long, consider shortening it to be bellow %d [total: %d, hash: %d, nodeName: %d]",
					cluster.ClusterInfo.GetName(), nodepool.GetName(), maxLength, clusterNameLength+nodeNameLength+baseLength, HashLength, nodeNameLength)
			}
		}
	}
	return nil
}
