package nodes

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	comm "github.com/berops/claudie/internal/command"
	"github.com/berops/claudie/internal/kubectl"
	"github.com/berops/claudie/internal/nodes"
	"github.com/berops/claudie/proto/pb/spec"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
)

const (
	ProviderIdFormat          = "claudie://%s"
	patchProviderIDPathFormat = "{\"spec\":{\"providerID\":\"%s\"}}"

	// number of concurrent workers patching nodes.
	workersLimit = 30
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
	kBase     kubectl.Kubectl
	clusterID string
	logger    zerolog.Logger

	errChan       chan error
	aggregateDone chan struct{}
	err           error

	wg errgroup.Group
}

func NewPatcher(cluster *spec.K8Scluster, logger zerolog.Logger) *Patcher {
	p := &Patcher{
		kBase: kubectl.Kubectl{
			Kubeconfig:        cluster.Kubeconfig,
			MaxKubectlRetries: 3,
		},
		clusterID: cluster.ClusterInfo.Id(),
		logger:    logger,

		errChan:       make(chan error),
		aggregateDone: make(chan struct{}),
	}

	p.wg.SetLimit(workersLimit)

	go func() {
		defer close(p.aggregateDone)
		for err := range p.errChan {
			p.err = errors.Join(p.err, err)
		}
	}()

	// No changes are made to the nodepools, or the nodes
	// thus all the patching can be done concurrently.
	for _, np := range cluster.ClusterInfo.NodePools {
		p.patchProviderID(np)
		p.annotateNodePool(np)
		p.labelNodePool(np)
		p.taintNodePool(np)
	}

	return p
}

func (p *Patcher) Wait() error {
	err := p.wg.Wait()
	close(p.errChan)
	<-p.aggregateDone
	return errors.Join(err, p.err) // combine the first returned error with any other errors.
}

func (p *Patcher) patchProviderID(np *spec.NodePool) {
	for _, node := range np.Nodes {
		nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))
		patchPath := fmt.Sprintf(patchProviderIDPathFormat, fmt.Sprintf(ProviderIdFormat, nodeName))

		kc := p.kBase
		kc.Stdout = comm.GetStdOut(p.clusterID)
		kc.Stderr = comm.GetStdErr(p.clusterID)

		p.wg.Go(func() error {
			if err := kc.KubectlPatch("node", nodeName, patchPath); err != nil {
				p.logger.Err(err).Str("node", nodeName).Msgf("Error while patching node with patch %s", patchPath)
				p.errChan <- fmt.Errorf("error while patching one or more nodes with providerID")
				// fallthrough
			}
			return nil
		})
	}
}

func (p *Patcher) labelNodePool(np *spec.NodePool) {
	name := np.Name
	nodeLabels, err := nodes.GetAllLabels(np, nil)
	if err != nil {
		p.errChan <- fmt.Errorf("failed to create labels for %s : %w", name, err)
		return
	}

	for key, value := range nodeLabels {
		patchPath, err := buildJSONPatchString("replace", "/metadata/labels/"+key, value)
		if err != nil {
			p.errChan <- fmt.Errorf("failed to create label %s patch path for nodepool %s: %w", key, name, err)
			continue
		}

		p.label(patchPath, np.Nodes)
	}
}

func (p *Patcher) label(patch string, nodes []*spec.Node) {
	for _, node := range nodes {
		nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))

		kc := p.kBase
		kc.Stdout = comm.GetStdOut(p.clusterID)
		kc.Stderr = comm.GetStdErr(p.clusterID)

		p.wg.Go(func() error {
			if err := kc.KubectlPatch("node", nodeName, patch, "--type", "json"); err != nil {
				p.logger.Err(err).Str("node", nodeName).Msgf("Failed to patch labels on node with path %s", patch)
				p.errChan <- fmt.Errorf("error while patching one or more nodes with labels")
				// fallthrough
			}
			return nil
		})
	}
}

func (p *Patcher) annotateNodePool(np *spec.NodePool) {
	isControl := np.IsControl
	zone := np.Zone()
	name := np.Name

	annotations := maps.Clone(np.Annotations)
	if annotations == nil {
		annotations = make(map[string]string)
	}
	// annotate worker nodes with provider spec name to match the storage classes
	// created in the SetupLonghorn step.
	// NOTE: the master nodes are by default set to NoSchedule, therefore we do not need to annotate them
	// If in the future, if add functionality to allow scheduling on master nodes, longhorn will need to add the annotation.
	if !isControl {
		k := "node.longhorn.io/default-node-tags"
		tags, ok := annotations[k]
		if !ok {
			tags = "[]"
		}
		var v []any
		if err := json.Unmarshal([]byte(tags), &v); err != nil {
			p.errChan <- fmt.Errorf("nodepool %s has invalid value for annotation %v, expected value to by of type array: %w", name, k, err)
			return
		}
		var found bool
		for i := range v {
			s, ok := v[i].(string)
			if !ok {
				continue
			}
			if s == zone {
				found = true
				break
			}
		}
		if !found {
			v = append(v, zone)
		}

		b, err := json.Marshal(v)
		if err != nil {
			p.errChan <- fmt.Errorf("failed to marshal modified annotations for nodepool %s: %w", name, err)
			return
		}
		annotations[k] = string(b)
	}

	patch, err := buildJSONAnnotationPatch(annotations)
	if err != nil {
		p.errChan <- fmt.Errorf("failed to create annotation for nodepool %s: %w", name, err)
		return
	}

	p.annotate(patch, np.Nodes)
}

func (p *Patcher) annotate(patch string, nodes []*spec.Node) {
	for _, node := range nodes {
		nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))

		kc := p.kBase
		kc.Stdout = comm.GetStdOut(p.clusterID)
		kc.Stderr = comm.GetStdErr(p.clusterID)

		p.wg.Go(func() error {
			if err := kc.KubectlPatch("node", nodeName, patch, "--type", "merge"); err != nil {
				p.logger.Err(err).Str("node", nodeName).Msgf("Failed to patch annotations on node %s", nodeName)
				p.errChan <- fmt.Errorf("error while applying annotations %v for node %s: %w", patch, nodeName, err)
				// fallthrough
			}
			return nil
		})
	}
}

func (p *Patcher) taintNodePool(np *spec.NodePool) {
	name := np.Name
	taints := nodes.GetAllTaints(np)

	patchPath, err := buildJSONPatchString("replace", "/spec/taints", taints)
	if err != nil {
		p.errChan <- fmt.Errorf("failed to create taints patch path for %s : %w", name, err)
		return
	}

	p.taint(patchPath, np.Nodes)
}

func (p *Patcher) taint(patchPath string, nodes []*spec.Node) {
	for _, node := range nodes {
		nodeName := strings.TrimPrefix(node.Name, fmt.Sprintf("%s-", p.clusterID))

		kc := p.kBase
		kc.Stdout = comm.GetStdOut(p.clusterID)
		kc.Stderr = comm.GetStdErr(p.clusterID)

		p.wg.Go(func() error {
			if err := kc.KubectlPatch("node", nodeName, patchPath, "--type", "json"); err != nil {
				p.logger.Err(err).Str("node", nodeName).Msgf("Failed to patch taints on node with path %s", patchPath)
				p.errChan <- fmt.Errorf("error while patching one or more nodes with taints")
				// fallthrough
			}
			return nil
		})
	}
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
