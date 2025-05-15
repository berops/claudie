package manifest

import (
	"fmt"
	"strconv"
)

const (
	maxLength  = 80 // total length of domain = 8 + len(publicIP)[15] + 19 + len(NodeName) + margin
	baseLength = 8 + 15 + 19
)

// checkLengthOfFutureDomain will check if the possible domain name is too long
// returns error if domain will be too long, nil if not
// Described in https://github.com/berops/claudie/issues/112#issuecomment-1015432224
func CheckLengthOfFutureDomain(m *Manifest) error {
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
	// <node-name> = nodeName + indexLength + 1 [separator]
	count := np.getCount(nodepoolName)
	if count == -1 {
		return fmt.Errorf("nodepool with %s name was not found, cannot validate the future domain", nodepoolName)
	}
	total := 1 /*separator*/ + len(nodepoolName) + len(strconv.Itoa(count)) /*get length of the string*/
	if total+baseLength > maxLength {
		return fmt.Errorf("nodepool name %s in cluster %s is too long, consider shortening it. Total node name cannot be longer than %d, the total for nodepool %s is %d",
			nodepoolName, clusterName, maxLength-baseLength, nodepoolName, total)
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
