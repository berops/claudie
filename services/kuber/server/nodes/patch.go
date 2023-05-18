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
	ProviderIdFormat = "claudie://%s"
	patchPathFormat  = "{\"spec\":{\"providerID\":\"%s\"}}"
)

type Patcher struct {
	clusterID string
	nodepools []*pb.NodePool
	kc        kubectl.Kubectl
}

func NewPatcher(cluster *pb.K8Scluster) *Patcher {
	kc := kubectl.Kubectl{Kubeconfig: cluster.Kubeconfig, MaxKubectlRetries: 3}
	clusterID := fmt.Sprintf("%s-%s", cluster.ClusterInfo.Name, cluster.ClusterInfo.Hash)
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		kc.Stdout = comm.GetStdOut(clusterID)
		kc.Stderr = comm.GetStdErr(clusterID)
	}
	return &Patcher{kc: kc, nodepools: cluster.ClusterInfo.NodePools, clusterID: clusterID}
}

func (p *Patcher) PatchProviderID(logger zerolog.Logger) error {
	var err error
	for _, np := range p.nodepools {
		for _, node := range np.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
			patchPath := fmt.Sprintf(patchPathFormat, fmt.Sprintf(ProviderIdFormat, nodeName))
			if err1 := p.kc.KubectlPatch("node", nodeName, patchPath); err1 != nil {
				logger.Err(err1).Str("node", nodeName).Msgf("Error while patching node with patch %s", patchPath)
				err = fmt.Errorf("error while patching one or more nodes")
			}
		}
	}
	return err
}
