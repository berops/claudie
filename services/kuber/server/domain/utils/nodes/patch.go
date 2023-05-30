package nodes

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb"
)

const (
	// TODO: By provider do we mean cloud-provider?
	ProviderIdFormat = "claudie://%s"
	patchPathFormat  = "{\"spec\":{\"providerID\":\"%s\"}}"
)

type NodePatcher struct {
	clusterID string
	nodepools []*pb.NodePool

	kc kubectl.Kubectl
}

// NewNodePatcher creates and configures an instance of NodePatcher.
func NewNodePatcher(cluster *pb.K8Scluster) *NodePatcher {
	kc := kubectl.Kubectl{Kubeconfig: cluster.Kubeconfig, MaxKubectlRetries: 3}

	clusterID := fmt.Sprintf("%s-%s", cluster.ClusterInfo.Name, cluster.ClusterInfo.Hash)
	if log.Logger.GetLevel() == zerolog.DebugLevel {

		kc.Stdout = comm.GetStdOut(clusterID)
		kc.Stderr = comm.GetStdErr(clusterID)
	}

	return &NodePatcher{
		clusterID: clusterID,
		nodepools: cluster.ClusterInfo.NodePools,

		kc: kc,
	}
}

// TODO: understand what this is doing and why is it doing so.
func (n *NodePatcher) PatchProviderID(logger zerolog.Logger) error {
	var err error

	for _, nodePool := range n.nodepools {
		for _, node := range nodePool.Nodes {
			var (
				nodeID    = strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", n.clusterID))
				patchPath = fmt.Sprintf(patchPathFormat, fmt.Sprintf(ProviderIdFormat, nodeID))
			)
			if patchErr := n.kc.KubectlPatch("node", nodeID, patchPath); patchErr != nil {
				logger.Err(patchErr).Str("node", nodeID).Msgf("Error while patching node with patch %s", patchPath)

				err = fmt.Errorf("error while patching one or more nodes")
			}
		}
	}

	return err
}
