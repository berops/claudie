package utils

import (
	"fmt"

	"github.com/Berops/platform/proto/pb"
)

func CheckLengthOfFutureDomain(config *pb.Config) error {
	// NOTE: In domain, we have .c.<gcpProject-id>.internal, and we cannot see what gcpProjectNameLength will be in future
	maxLength := 37    // total length of domain = clusterName + hash + nodeName + indexLength + separators
	currentLength := 1 // "-" separator between clusterName and hash
	desiredState := config.DesiredState

	for _, cluster := range desiredState.GetClusters() {
		currentLength += len(cluster.ClusterInfo.Name) + HashLength
		for _, nodepool := range cluster.ClusterInfo.GetNodePools() {
			nodeNameLength := 1  // "-" separator between hash and nodeName
			nodeIndexLength := 1 // "-" separator between nodeName and index
			nodeIndexLength += len(fmt.Sprint(nodepool.Count))
			nodeNameLength += nodeIndexLength
			if maxLength <= currentLength+nodeNameLength {
				return fmt.Errorf("cluster name %s or nodepool name %s is too long, consider shortening it to be bellow %d [total: %d, hash: %d, nodeName: %d]", cluster.ClusterInfo.GetName(), nodepool.GetName(), maxLength, currentLength+nodeNameLength, HashLength, nodeNameLength)
			}
		}
	}
	return nil
}
