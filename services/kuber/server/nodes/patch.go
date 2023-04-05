package nodes

import (
	"fmt"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/proto/pb"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	kc := kubectl.Kubectl{Kubeconfig: cluster.Kubeconfig}
	clusterID := fmt.Sprintf("%s-%s", cluster.ClusterInfo.Name, cluster.ClusterInfo.Hash)
	if log.Logger.GetLevel() == zerolog.DebugLevel {
		kc.Stdout = comm.GetStdOut(clusterID)
		kc.Stderr = comm.GetStdErr(clusterID)
	}
	return &Patcher{kc: kc, nodepools: cluster.ClusterInfo.NodePools, clusterID: clusterID}
}

func (p *Patcher) PatchProviderID() error {
	var err error
	for _, np := range p.nodepools {
		for _, node := range np.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
			patchPath := fmt.Sprintf(patchPathFormat, fmt.Sprintf(ProviderIdFormat, nodeName))
			if err1 := p.kc.KubectlPatch("node", nodeName, patchPath); err1 != nil {
				log.Error().Msgf("Error while patching node %s with patch %s : %v", nodeName, patchPath, err1)
				err = fmt.Errorf("error while patching one or more nodes")
			}
		}
	}
	return err
}
