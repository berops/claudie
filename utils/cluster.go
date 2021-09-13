package utils // import "github.com/Berops/platform/utils"

import "github.com/Berops/platform/proto/pb"

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
