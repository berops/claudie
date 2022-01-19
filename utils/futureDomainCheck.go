package utils

import (
	"fmt"

	"github.com/Berops/platform/proto/pb"
)

const gcpNodeNameLength = 8
const hetznerNodeNameLength = 12

func CheckLengthOfFutureDomain(config *pb.Config, hashLength int) error {
	// NOTE: after that we have .c.<gcpProject-id>.internal, and we cannot see what gcpProjectLength will be used in future
	maxLenght := 37    // total length of domain = clusterName + hash + nodeName + indexLength + separators
	currentLenght := 1 // "-" separator between clusterName and hash
	desiredState := config.DesiredState

	for _, cluster := range desiredState.GetClusters() {
		currentLenght += len(cluster.Name) + hashLength
		for _, nodepool := range cluster.GetNodePools() {
			nodeNameLength := 1  // "-" separator between hash and nodeName
			nodeIndexLength := 1 // "-" separator between nodeName and index
			if nodepool.Master.Count > nodepool.Worker.Count {
				nodeIndexLength += len(fmt.Sprint(nodepool.Master.Count))
			} else {
				nodeIndexLength += len(fmt.Sprint(nodepool.Worker.Count))
			}
			if nodepool.Provider.Name == "gcp" {
				nodeNameLength += gcpNodeNameLength + nodeIndexLength
			} else if nodepool.Provider.Name == "hetzner" {
				nodeNameLength += hetznerNodeNameLength + nodeIndexLength
			}
			if maxLenght < currentLenght+len(cluster.GetName())+nodeNameLength {
				return fmt.Errorf("cluster name %s or nodepool name %s is too long, consider shortening it", cluster.GetName(), nodepool.GetName())
			}
		}
	}
	return nil
}
