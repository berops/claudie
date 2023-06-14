package nodes

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodes"
	"github.com/berops/claudie/proto/pb"
)

const (
	ProviderIdFormat          = "claudie://%s"
	patchProviderIDPathFormat = "{\"spec\":{\"providerID\":\"%s\"}}"
	patchTaintsPath           = "{\"spec\":{\"taints\":[%s]}}"
	patchLabelsPath           = "{\"metadata\":{\"labels\":{%s}}}"
)

type Patcher struct {
	clusterID        string
	desiredNodepools []*pb.NodePool
	kc               kubectl.Kubectl
}

func NewPatcher(cluster *pb.K8Scluster) *Patcher {
	kc := kubectl.Kubectl{Kubeconfig: cluster.Kubeconfig, MaxKubectlRetries: 3}
	clusterID := fmt.Sprintf("%s-%s", cluster.ClusterInfo.Name, cluster.ClusterInfo.Hash)

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		kc.Stdout = comm.GetStdOut(clusterID)
		kc.Stderr = comm.GetStdErr(clusterID)
	}

	return &Patcher{kc: kc, desiredNodepools: cluster.ClusterInfo.NodePools, clusterID: clusterID}
}

func (p *Patcher) PatchProviderID(logger zerolog.Logger) error {
	var err error
	for _, np := range p.desiredNodepools {
		for _, node := range np.GetNodes() {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
			patchPath := fmt.Sprintf(patchProviderIDPathFormat, fmt.Sprintf(ProviderIdFormat, nodeName))
			if err1 := p.kc.KubectlPatch("node", nodeName, patchPath); err1 != nil {
				logger.Err(err1).Str("node", nodeName).Msgf("Error while patching node with patch %s", patchPath)
				err = fmt.Errorf("error while patching one or more nodes")
			}
		}
	}
	return err
}

func (p *Patcher) PatchLabels(logger zerolog.Logger) error {
	var err error
	for _, np := range p.desiredNodepools {
		patchPath := fmt.Sprintf(patchLabelsPath, buildLabelString(np))
		for _, node := range np.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
			if err1 := p.kc.KubectlPatch("node", nodeName, patchPath, "--type", "merge"); err1 != nil {
				logger.Err(err1).Str("node", nodeName).Msgf("Failed to patch labels on node with path %s", patchPath)
				err = fmt.Errorf("error while patching one or more nodes")
			}
		}
	}
	return err
}

func (p *Patcher) PatchTaints(logger zerolog.Logger) error {
	var err error
	for _, np := range p.desiredNodepools {
		patchPath := fmt.Sprintf(patchTaintsPath, buildTaintString(np))
		for _, node := range np.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
			if err1 := p.kc.KubectlPatch("node", nodeName, patchPath, "--type", "merge"); err1 != nil {
				logger.Err(err1).Str("node", nodeName).Msgf("Failed to patch taints on node with path %s", patchPath)
				err = fmt.Errorf("error while patching one or more nodes")
			}
		}
	}
	return err
}

func buildTaintString(np *pb.NodePool) string {
	var sb strings.Builder
	for _, t := range nodes.GetAllTaints(np) {
		sb.WriteString(fmt.Sprintf("{\"effect\":\"%s\",\"key\":\"%s\",\"value\":\"%s\"},", t.Effect, t.Key, t.Value))
	}
	return strings.TrimRight(sb.String(), ",")
}

func buildLabelString(np *pb.NodePool) string {
	var sb strings.Builder
	for k, v := range nodes.GetAllLabels(np) {
		sb.WriteString(fmt.Sprintf("\"%s\":\"%s\",", k, v))
	}
	return strings.TrimRight(sb.String(), ",")
}
