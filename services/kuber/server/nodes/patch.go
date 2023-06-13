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
	ProviderIdFormat          = "claudie://%s"
	patchProviderIDPathFormat = "{\"spec\":{\"providerID\":\"%s\"}}"
)

type Patcher struct {
	clusterID        string
	desiredNodepools []*pb.NodePool
	currentNodepools []*pb.NodePool
	kc               kubectl.Kubectl
}

func NewPatcher(desiredCluster, currentCluster *pb.K8Scluster) *Patcher {
	kc := kubectl.Kubectl{Kubeconfig: desiredCluster.Kubeconfig, MaxKubectlRetries: 3}
	clusterID := fmt.Sprintf("%s-%s", desiredCluster.ClusterInfo.Name, desiredCluster.ClusterInfo.Hash)

	if log.Logger.GetLevel() == zerolog.DebugLevel {
		kc.Stdout = comm.GetStdOut(clusterID)
		kc.Stderr = comm.GetStdErr(clusterID)
	}

	return &Patcher{kc: kc, currentNodepools: currentCluster.ClusterInfo.NodePools, desiredNodepools: desiredCluster.ClusterInfo.NodePools, clusterID: clusterID}
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
	deleteLabels := p.getLabelsToDelete()
	// Delete labels from nodes
	for _, np := range p.desiredNodepools {
		dl := deleteLabels[np.Name]
		if len(dl) == 0 {
			continue
		}
		labels := mergeDeletedLabels(dl)
		for _, node := range np.Nodes {
			if err1 := p.kc.KubectlLabel("node", node.Name, labels); err != nil {
				logger.Err(err1).Str("node", node.Name).Msgf("Error while removing labels \"%s\"", labels)
				err = fmt.Errorf("error while labeling one or more nodes")
			}
		}
	}
	// Apply labels to nodes
	for _, np := range p.desiredNodepools {
		labels := mergeAppliedLabels(np.Labels)
		for _, node := range np.Nodes {
			if err1 := p.kc.KubectlLabel("node", node.Name, labels, "--overwrite"); err != nil {
				logger.Err(err1).Str("node", node.Name).Msgf("Error while applying labels \"%s\"", labels)
				err = fmt.Errorf("error while labeling one or more nodes")
			}
		}
	}
	return err
}

func (p *Patcher) PatchTaints(logger zerolog.Logger) error {
	var err error
	deleteTaints := p.getTaintsToDelete()
	// Delete taints from nodes
	for _, np := range p.desiredNodepools {
		dl := deleteTaints[np.Name]
		if len(dl) == 0 {
			continue
		}
		taints := mergeDeletedTaints(dl)
		for _, node := range np.Nodes {
			if err1 := p.kc.KubectlTaint("node", node.Name, taints); err != nil {
				logger.Err(err1).Str("node", node.Name).Msgf("Error while removing taints \"%s\"", taints)
				err = fmt.Errorf("error while tainting one or more nodes")
			}
		}
	}
	// Apply taints to nodes
	for _, np := range p.desiredNodepools {
		taints := mergeAppliedTaints(np.Taints)
		for _, node := range np.Nodes {
			if err1 := p.kc.KubectlTaint("node", node.Name, taints, "--overwrite"); err != nil {
				logger.Err(err1).Str("node", node.Name).Msgf("Error while applying taints \"%s\"", taints)
				err = fmt.Errorf("error while tainting one or more nodes")
			}
		}
	}
	return err
}

func (p *Patcher) getLabelsToDelete() map[string][]string {
	delete := make(map[string][]string)
dnp:
	for _, dnp := range p.desiredNodepools {
		for _, cnp := range p.currentNodepools {
			if dnp.Name == cnp.Name {
				for ck := range cnp.Labels {
					if _, ok := dnp.Labels[ck]; !ok {
						delete[dnp.Name] = append(delete[dnp.Name], ck)
					}
				}
				continue dnp
			}
		}
	}
	return delete
}

func mergeDeletedLabels(l []string) string {
	var sb strings.Builder
	for _, s := range l {
		sb.WriteString(fmt.Sprintf("%s- ", s))
	}
	return sb.String()
}

func mergeAppliedLabels(l map[string]string) string {
	var sb strings.Builder
	for k, v := range l {
		sb.WriteString(fmt.Sprintf("%s=%s ", k, v))
	}
	return sb.String()
}

func (p *Patcher) getTaintsToDelete() map[string][]*pb.Taint {

}

func mergeDeletedTaints(t []*pb.Taint) string {
	var sb strings.Builder
	for _, s := range t {
		sb.WriteString(fmt.Sprintf("%s=%s:%s- ", s.Key, s.Value, s.Effect))
	}
	return sb.String()
}

func mergeAppliedTaints(t []*pb.Taint) string {
	var sb strings.Builder
	for _, s := range t {
		sb.WriteString(fmt.Sprintf("%s=%s:%s ", s.Key, s.Value, s.Effect))
	}
	return sb.String()
}
