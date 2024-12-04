package nodes

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodes"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
)

const (
	ProviderIdFormat          = "claudie://%s"
	patchProviderIDPathFormat = "{\"spec\":{\"providerID\":\"%s\"}}"
)

type patchJson struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type PatchAnnotations struct {
	MetadataAnnotations `json:"metadata"`
}

type MetadataAnnotations struct {
	Annotations map[string]string `json:"annotations"`
}

type Patcher struct {
	clusterID        string
	desiredNodepools []*spec.NodePool
	kc               kubectl.Kubectl
	logger           zerolog.Logger
}

func NewPatcher(cluster *spec.K8Scluster, logger zerolog.Logger) *Patcher {
	kc := kubectl.Kubectl{Kubeconfig: cluster.Kubeconfig, MaxKubectlRetries: 3}

	clusterID := cluster.ClusterInfo.Id()
	kc.Stdout = comm.GetStdOut(clusterID)
	kc.Stderr = comm.GetStdErr(clusterID)

	return &Patcher{
		kc:               kc,
		desiredNodepools: cluster.ClusterInfo.NodePools,
		clusterID:        clusterID,
		logger:           logger,
	}
}

func (p *Patcher) PatchProviderID() error {
	var err error
	for _, np := range p.desiredNodepools {
		for _, node := range np.GetNodes() {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
			patchPath := fmt.Sprintf(patchProviderIDPathFormat, fmt.Sprintf(ProviderIdFormat, nodeName))
			if err1 := p.kc.KubectlPatch("node", nodeName, patchPath); err1 != nil {
				p.logger.Err(err1).Str("node", nodeName).Msgf("Error while patching node with patch %s", patchPath)
				err = fmt.Errorf("error while patching one or more nodes with providerID")
			}
		}
	}
	return err
}

func (p *Patcher) PatchLabels() error {
	var err error
	for _, np := range p.desiredNodepools {
		nodeLabels, err1 := nodes.GetAllLabels(np, nil)
		if err1 != nil {
			return fmt.Errorf("failed to create labels for %s : %w, %w", np.Name, err, err1)
		}

		for _, node := range np.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
			for key, value := range nodeLabels {
				patchPath, err1 := buildJSONPatchString("replace", "/metadata/labels/"+key, value)
				if err1 != nil {
					return fmt.Errorf("failed to create label %s patch path for %s : %w, %w", key, np.Name, err, err1)
				}
				if err1 := p.kc.KubectlPatch("node", nodeName, patchPath, "--type", "json"); err1 != nil {
					p.logger.Err(err1).Str("node", nodeName).Msgf("Failed to patch labels on node with path %s", patchPath)
					err = fmt.Errorf("error while patching one or more nodes with labels")
				}
			}
		}
	}
	return err
}

func (p *Patcher) PatchAnnotations() error {
	var errAll error
	for _, np := range p.desiredNodepools {
		annotations := np.Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}
		// annotate worker nodes with provider spec name to match the storage classes
		// created in the SetupLonghorn step.
		// NOTE: the master nodes are by default set to NoSchedule, therefore we do not need to annotate them
		// If in the future, if add functionality to allow scheduling on master nodes, longhorn will need to add the annotation.
		if !np.IsControl {
			k := "node.longhorn.io/default-node-tags"
			tags, ok := annotations[k]
			if !ok {
				tags = "[]"
			}
			var v []any
			if err := json.Unmarshal([]byte(tags), &v); err != nil {
				errAll = errors.Join(errAll, fmt.Errorf("nodepool %s has invalid value for annotation %v, expected value to by of type array: %w", np.Name, k, err))
				continue
			}
			var found bool
			for i := range v {
				s, ok := v[i].(string)
				if !ok {
					continue
				}
				if s == np.Zone() {
					found = true
					break
				}
			}
			if !found {
				v = append(v, np.Zone())
			}

			b, err := json.Marshal(v)
			if err != nil {
				errAll = errors.Join(errAll, fmt.Errorf("failed to marshal modified annotations for nodepool %s: %w", np.Name, err))
				continue
			}
			annotations[k] = string(b)
		}

		for _, node := range np.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
			patch, err := buildJSONAnnotationPatch(annotations)
			if err != nil {
				errAll = errors.Join(errAll, fmt.Errorf("failed to create annotation for node %s: %w", nodeName, err))
				continue
			}
			if err := p.kc.KubectlPatch("node", nodeName, patch, "--type", "merge"); err != nil {
				errAll = errors.Join(err, fmt.Errorf("error while applying annotations %v for node %s: %w", annotations, nodeName, err))
				continue
			}
		}
	}
	return errAll
}

func (p *Patcher) PatchTaints() error {
	var err error
	for _, np := range p.desiredNodepools {
		patchPath, err1 := buildJSONPatchString("replace", "/spec/taints", nodes.GetAllTaints(np))
		if err1 != nil {
			return fmt.Errorf("failed to create taints patch path for %s : %w", np.Name, err)
		}
		for _, node := range np.Nodes {
			nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
			if err1 := p.kc.KubectlPatch("node", nodeName, patchPath, "--type", "json"); err1 != nil {
				p.logger.Err(err1).Str("node", nodeName).Msgf("Failed to patch taints on node with path %s", patchPath)
				err = fmt.Errorf("error while patching one or more nodes with taints")
			}
		}
	}
	return err
}

func buildJSONAnnotationPatch(data map[string]string) (string, error) {
	metadata := PatchAnnotations{
		MetadataAnnotations{
			Annotations: data,
		},
	}
	jsonPatch, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return string(jsonPatch), nil
}

func buildJSONPatchString(op, path string, value any) (string, error) {
	patchJson := patchJson{Op: op, Path: path, Value: value}
	b, err := json.Marshal(patchJson)
	if err != nil {
		return "", err
	}
	return "[" + string(b) + "]", nil
}
