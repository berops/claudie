package manifest

import (
	"fmt"

	"github.com/Berops/claudie/internal/utils"
)

const (
	maxLength  = 80 // total length of domain = 8 + len(publicIP)[15] + 19 + len(NodeName) + margin
	baseLength = 8 + 19 + 15
)

// CheckLengthOfFutureDomain will check if the possible domain name is too long
// returns error if domain will be too long, nil if not
// Described in https://github.com/Berops/claudie/issues/112#issuecomment-1015432224
func checkLengthOfFutureDomain(m *Manifest) error {
	// https://<public-ip>:6443/<api-path>/<node-name>
	// <node-name> = clusterName + hash + nodeName + indexLength + separators
	for _, cluster := range m.Kubernetes.Clusters {
		for _, nodepoolName := range cluster.Pools.Control {
			if total, err := m.NodePools.checkNodepoolDomain(nodepoolName, cluster.Name); err != nil {
				return fmt.Errorf("cluster name %s or nodepool name %s is too long, consider shortening it to be bellow total node name bellow %d [total node name: %d, hash: %d]",
					cluster.Name, nodepoolName, maxLength, total, utils.HashLength)
			}
		}
		for _, nodepoolName := range cluster.Pools.Control {
			if total, err := m.NodePools.checkNodepoolDomain(nodepoolName, cluster.Name); err != nil {
				return fmt.Errorf("cluster name %s or nodepool name %s is too long, consider shortening it to be bellow total node name bellow %d [total node name: %d, hash: %d]",
					cluster.Name, nodepoolName, maxLength, total, utils.HashLength)
			}
		}
	}

	return nil
}

func (np *NodePool) checkNodepoolDomain(NodePoolName, clusterName string) (int, error) {
	return 0, nil
}
