package manifest

import (
	"fmt"
	"strconv"

	"github.com/Berops/claudie/internal/utils"
)

const (
	maxLength  = 80 // total length of domain = 8 + len(publicIP)[15] + 19 + len(NodeName) + margin
	baseLength = 8 + 19 + 15
)

// checkLengthOfFutureDomain will check if the possible domain name is too long
// returns error if domain will be too long, nil if not
// Described in https://github.com/Berops/claudie/issues/112#issuecomment-1015432224
func checkLengthOfFutureDomain(m *Manifest) error {
	// https://<public-ip>:6443/<api-path>/<node-name>
	for _, cluster := range m.Kubernetes.Clusters {
		for _, nodepoolName := range cluster.Pools.Control {
			if err := m.NodePools.checkNodepoolDomain(nodepoolName, cluster.Name); err != nil {
				return err
			}
		}
		for _, nodepoolName := range cluster.Pools.Compute {
			if err := m.NodePools.checkNodepoolDomain(nodepoolName, cluster.Name); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkNodepoolDomain check the future domain for the nodepool specified in nodepoolName parameter
// returns nil if domain is not too long, error otherwise
func (np *NodePool) checkNodepoolDomain(nodepoolName, clusterName string) error {
	// <node-name> = clusterName + hash + nodeName + indexLength + separators
	count := np.getCount(nodepoolName)
	if count == -1 {
		return fmt.Errorf("nodepool with %s name was not found, cannot validate the future domain", nodepoolName)
	}
	total := 3 /*separator*/ + len(clusterName) + utils.HashLength + len(nodepoolName) + len(strconv.Itoa(count)) /*get length of the string*/
	if total > maxLength {
		return fmt.Errorf("cluster name %s or nodepool name %s is too long, consider shortening it to be bellow total node name bellow %d [total node name: %d, hash: %d]",
			clusterName, nodepoolName, maxLength, total, utils.HashLength)
	}
	return nil
}

// getCount returns the count of the specified nodepool
// for dynamic nodepools, returns the Count parameter
// for static nodepools, returns length of the Nodes slice
// if nodepool was not found, returns -1
func (np *NodePool) getCount(nodepoolName string) int {
	for _, n := range np.Dynamic {
		if n.Name == nodepoolName {
			return int(n.Count)
		}
	}
	for _, n := range np.Static {
		if n.Name == nodepoolName {
			return len(n.Nodes)
		}
	}
	return -1
}
