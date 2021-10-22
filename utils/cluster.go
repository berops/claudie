package utils

import (
	"crypto/md5"

	"github.com/Berops/platform/proto/pb"
)

// GetClusterByName will return Cluster that will have same name as specified in parameters
// If no name is found, return nil
func GetClusterByName(clusterName string, clusters []*pb.Cluster) *pb.Cluster {
	if clusterName == "" {
		return nil
	}

	if len(clusters) == 0 {
		return nil
	}

	for _, cluster := range clusters {
		if cluster.Name == clusterName {
			return cluster
		}
	}

	return nil
}

// Calculate md5 hash of the input string arg
func CalcChecksum(data string) []byte {
	res := md5.Sum([]byte(data))
	// Creating a slice using an array you can just make a simple slice expression
	return res[:]
}
